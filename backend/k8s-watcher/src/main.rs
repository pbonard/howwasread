//! Watches pods, warning events, nodes and operator resources, records every problem and alerts on the important ones.

use std::fmt::Debug;
use std::str::FromStr;

use futures::StreamExt;
use k8s_openapi::api::core::v1::{Event as CoreEvent, Node, Pod};
use kube::api::{ApiResource, DynamicObject, GroupVersionKind};
use kube::runtime::{WatchStreamExt, watcher};
use kube::{Api, Client, Resource};
use serde::de::DeserializeOwned;
use tokio::sync::mpsc;
use tokio::task::JoinSet;
use tracing::{info, warn};

use k8s_watcher::health;
use k8s_watcher::monitor::{Monitor, Msg, Settings};
use k8s_watcher::notify::Notifier;

fn env_or<T: FromStr>(name: &str, default: T) -> T {
    std::env::var(name)
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(default)
}

/// Sends every event of one resource type to the monitor. The watcher lists, then watches, and lists again after
/// an expired watch or an error, the backoff spaces out those retries.
async fn forward<K>(
    name: &'static str,
    api: Api<K>,
    config: watcher::Config,
    tx: mpsc::Sender<Msg>,
    wrap: impl Fn(watcher::Event<K>) -> Msg,
) where
    K: Resource + Clone + DeserializeOwned + Debug + Send + 'static,
{
    let mut events = std::pin::pin!(watcher(api, config).default_backoff());
    while let Some(event) = events.next().await {
        match event {
            Ok(event) => {
                if tx.send(wrap(event)).await.is_err() {
                    return;
                }
            }
            Err(e) => warn!("{name} watch restarts after error: {e}"),
        }
    }
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    // logs are JSON lines on stdout like the records and the go services, so every line parses the same way in Loki
    tracing_subscriber::fmt()
        .json()
        .with_writer(std::io::stdout)
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env().unwrap_or_else(|_| "info".into()),
        )
        .init();

    // the in-cluster service account when running in a pod, otherwise the current kubeconfig
    let client = Client::try_default().await?;
    let settings = Settings {
        crd_grace: env_or("CRD_GRACE_SECONDS", 120.0),
        cluster_restart_min: env_or("CLUSTER_RESTART_MIN", 5),
        log_tail_lines: env_or("LOG_TAIL_LINES", 30),
    };

    let (tx, rx) = mpsc::channel(1024);
    let mut tasks = JoinSet::new();
    tasks.spawn(forward(
        "pods",
        Api::<Pod>::all(client.clone()),
        watcher::Config::default(),
        tx.clone(),
        |e| Msg::Pod(Box::new(e)),
    ));
    tasks.spawn(forward(
        "events",
        Api::<CoreEvent>::all(client.clone()),
        watcher::Config::default().fields("type=Warning"),
        tx.clone(),
        |e| Msg::Warning(Box::new(e)),
    ));
    tasks.spawn(forward(
        "nodes",
        Api::<Node>::all(client.clone()),
        watcher::Config::default(),
        tx.clone(),
        |e| Msg::Node(Box::new(e)),
    ));
    for watched in &health::RESOURCES {
        let resource = ApiResource::from_gvk_with_plural(
            &GroupVersionKind::gvk(watched.group, watched.version, watched.kind),
            watched.plural,
        );
        let (plural, check) = (watched.plural, watched.check);
        tasks.spawn(forward(
            plural,
            Api::<DynamicObject>::all_with(client.clone(), &resource),
            watcher::Config::default(),
            tx.clone(),
            move |event| Msg::Resource {
                plural,
                check,
                event: Box::new(event),
            },
        ));
    }
    info!(
        "watching pods, events, nodes, {}",
        health::RESOURCES
            .iter()
            .map(|r| r.plural)
            .collect::<Vec<_>>()
            .join(", ")
    );

    let monitor = Monitor::new(client, Notifier::spawn(), settings, tx);
    // the watch tasks only end when the monitor is gone, so either one ending is a bug, and exiting lets kubernetes
    // restart the pod
    tokio::select! {
        _ = monitor.run(rx) => anyhow::bail!("monitor stopped"),
        Some(ended) = tasks.join_next() => anyhow::bail!("a watch task stopped: {ended:?}"),
    }
}
