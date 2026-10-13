use std::fmt::Debug;
use std::time::Duration;

use anyhow::Context;
use futures::StreamExt;
use k8s_openapi::api::coordination::v1::{Lease, LeaseSpec};
use k8s_openapi::api::core::v1::{Event as CoreEvent, Node, Pod};
use k8s_openapi::apimachinery::pkg::apis::meta::v1::{MicroTime, ObjectMeta};
use k8s_openapi::jiff::Timestamp;
use kube::api::{Patch, PatchParams};
use kube::runtime::{WatchStreamExt, watcher};
use kube::{Api, Client, Resource};
use serde::de::DeserializeOwned;
use tokio::sync::{mpsc, watch};
use tokio::task::JoinSet;
use tracing::{info, warn};

use k8s_watcher::monitor::{
    FAILURE_GAP, HEARTBEAT_SECONDS, MAX_CATCHUP, Monitor, Msg, Settings, Watch,
};
use k8s_watcher::notify::Notifier;
use k8s_watcher::util::{env_or, now};

const HEARTBEAT_LEASE: &str = "k8s-watcher";
const FIRST_START_CATCHUP: f64 = 300.0;
const WATCH_IDLE_TIMEOUT: f64 = 295.0;

async fn forward<K>(
    watch: Watch,
    api: Api<K>,
    config: watcher::Config,
    startup_since: f64,
    tx: mpsc::Sender<Msg>,
    wrap: impl Fn(watcher::Event<K>) -> Msg,
) where
    K: Resource + Clone + DeserializeOwned + Debug + Send + 'static,
{
    let mut events = std::pin::pin!(watcher(api, config).default_backoff());
    let mut first_list_done = false;
    let mut failing: Option<(f64, f64)> = None;
    while let Some(event) = events.next().await {
        let event = match event {
            Ok(event) => event,
            Err(e) => {
                warn!("{watch:?} watch restarts after error: {e}");
                let at = now();
                failing = Some(match failing {
                    Some((first, last)) if at - last <= FAILURE_GAP => (first, at),
                    _ => (at, at),
                });
                if tx.send(Msg::Failed { watch }).await.is_err() {
                    return;
                }
                continue;
            }
        };
        match event {
            watcher::Event::Init => {
                let since = match failing {
                    Some((first, _)) if first_list_done => {
                        (first - WATCH_IDLE_TIMEOUT).max(now() - MAX_CATCHUP)
                    }
                    _ => startup_since,
                };
                if tx.send(Msg::Listing { watch, since }).await.is_err() {
                    return;
                }
            }
            watcher::Event::InitDone => {
                first_list_done = true;
                failing = None;
            }
            watcher::Event::Apply(_) | watcher::Event::Delete(_) => failing = None,
            watcher::Event::InitApply(_) => {}
        }
        if tx.send(wrap(event)).await.is_err() {
            return;
        }
    }
}

async fn heartbeat(leases: Api<Lease>, holder: String, caught_up: watch::Receiver<bool>) {
    let mut tick = tokio::time::interval(Duration::from_secs(HEARTBEAT_SECONDS));
    loop {
        tick.tick().await;
        if !*caught_up.borrow() {
            continue;
        }
        let lease = Lease {
            metadata: ObjectMeta {
                name: Some(HEARTBEAT_LEASE.into()),
                ..Default::default()
            },
            spec: Some(LeaseSpec {
                holder_identity: Some(holder.clone()),
                renew_time: Some(MicroTime(Timestamp::now())),
                lease_duration_seconds: Some(3 * HEARTBEAT_SECONDS as i32),
                ..Default::default()
            }),
        };
        let params = PatchParams::apply("k8s-watcher").force();
        if let Err(e) = leases
            .patch(HEARTBEAT_LEASE, &params, &Patch::Apply(&lease))
            .await
        {
            warn!("fail to renew the heartbeat lease: {e}");
        }
    }
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .json()
        .with_writer(std::io::stdout)
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env().unwrap_or_else(|_| "info".into()),
        )
        .init();

    let client = Client::try_default().await?;
    let leases: Api<Lease> = Api::default_namespaced(client.clone());
    let last_alive = leases
        .get_opt(HEARTBEAT_LEASE)
        .await
        .context("fail to read the heartbeat lease")?
        .and_then(|l| l.spec?.renew_time)
        .map(|t| t.0.as_second() as f64);
    let startup_since = last_alive
        .map_or(now() - FIRST_START_CATCHUP, |t| {
            t - HEARTBEAT_SECONDS as f64 - WATCH_IDLE_TIMEOUT
        })
        .max(now() - MAX_CATCHUP);
    let settings = Settings {
        log_tail_lines: env_or("LOG_TAIL_LINES", 30),
    };

    let (tx, rx) = mpsc::channel(1024);
    let (caught_up_tx, caught_up) = watch::channel(false);
    let mut tasks = JoinSet::new();
    tasks.spawn(forward(
        Watch::Pods,
        Api::<Pod>::all(client.clone()),
        watcher::Config::default(),
        startup_since,
        tx.clone(),
        |e| Msg::Pod(Box::new(e)),
    ));
    tasks.spawn(forward(
        Watch::Events,
        Api::<CoreEvent>::all(client.clone()),
        watcher::Config::default().fields("type=Warning"),
        startup_since,
        tx.clone(),
        |e| Msg::Warning(Box::new(e)),
    ));
    tasks.spawn(forward(
        Watch::Nodes,
        Api::<Node>::all(client.clone()),
        watcher::Config::default(),
        startup_since,
        tx.clone(),
        |e| Msg::Node(Box::new(e)),
    ));
    tasks.spawn(heartbeat(
        leases,
        std::env::var("HOSTNAME").unwrap_or_else(|_| "k8s-watcher".into()),
        caught_up,
    ));
    info!(
        "watching pods, events, nodes, reporting what happened since {}",
        Timestamp::from_second(startup_since as i64)
            .map(|t| t.to_string())
            .unwrap_or_default()
    );

    let monitor = Monitor::new(
        client,
        Notifier::spawn(tx.clone()),
        settings,
        tx,
        caught_up_tx,
    );
    tokio::select! {
        _ = monitor.run(rx) => anyhow::bail!("monitor stopped"),
        Some(ended) = tasks.join_next() => anyhow::bail!("a watch task stopped: {ended:?}"),
    }
}
