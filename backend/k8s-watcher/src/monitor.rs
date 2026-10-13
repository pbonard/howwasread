mod events;
mod nodes;
mod pods;

use std::collections::{HashMap, HashSet, VecDeque};
use std::time::Duration;

use k8s_openapi::api::core::v1::{Event as CoreEvent, Node, Pod};
use kube::runtime::watcher::Event;
use kube::Client;
use serde::Serialize;
use tokio::sync::{mpsc, watch};
use tokio::time::interval;
use tracing::info;

use crate::notify::{Notifier, Severity};
use crate::util::now;

const GROUP_WINDOW: f64 = 30.0;
pub const HEARTBEAT_SECONDS: u64 = 10;
pub const MAX_CATCHUP: f64 = 86400.0;
pub const FAILURE_GAP: f64 = 120.0;

/// (namespace, pod, container)
pub type ContainerKey = (String, String, String);

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Watch {
    Pods,
    Events,
    Nodes,
}

const WATCHES: [Watch; 3] = [Watch::Pods, Watch::Events, Watch::Nodes];

#[derive(Default)]
struct WatchState {
    listed: bool,
    failed_at: Option<f64>,
}

pub enum Msg {
    Listing { watch: Watch, since: f64 },
    Failed { watch: Watch },
    Pod(Box<Event<Pod>>),
    Warning(Box<Event<CoreEvent>>),
    Node(Box<Event<Node>>),
    Restart(Box<Restart>),
    Sent { keys: Vec<String>, sent: bool },
}

pub struct Settings {
    pub log_tail_lines: i64,
}

#[derive(Serialize)]
pub struct Restart {
    namespace: String,
    pod: String,
    container: String,
    node: Option<String>,
    exit_code: i32,
    reason: Option<String>,
    started_at: Option<String>,
    finished_at: Option<String>,
    restart_count: i32,
    memory_limit: Option<f64>,
    last_logs: String,
    #[serde(skip)]
    alert_key: String,
}

pub struct Monitor {
    client: Client,
    notifier: Notifier,
    settings: Settings,
    tx: mpsc::Sender<Msg>,
    restart_counts: HashMap<ContainerKey, i32>,
    restart_times: HashMap<ContainerKey, VecDeque<f64>>,
    crash_loop_alerted: HashMap<ContainerKey, f64>,
    pending: Vec<Restart>,
    node_conditions: HashMap<(String, String), String>,
    listed: Option<HashSet<ContainerKey>>,
    pods_since: f64,
    events_since: f64,
    nodes_since: f64,
    alerted: HashMap<String, f64>,
    sending: HashSet<String>,
    watches: HashMap<Watch, WatchState>,
    fetching: usize,
    catching_up: bool,
    caught_up: watch::Sender<bool>,
}

impl Monitor {
    pub fn new(
        client: Client,
        notifier: Notifier,
        settings: Settings,
        tx: mpsc::Sender<Msg>,
        caught_up: watch::Sender<bool>,
    ) -> Self {
        Self {
            client,
            notifier,
            settings,
            tx,
            restart_counts: HashMap::new(),
            restart_times: HashMap::new(),
            crash_loop_alerted: HashMap::new(),
            pending: Vec::new(),
            node_conditions: HashMap::new(),
            listed: None,
            pods_since: now(),
            events_since: now(),
            nodes_since: now(),
            alerted: HashMap::new(),
            sending: HashSet::new(),
            watches: HashMap::new(),
            fetching: 0,
            catching_up: true,
            caught_up,
        }
    }

    fn is_alerted(&self, key: &str) -> bool {
        self.alerted.contains_key(key) || self.sending.contains(key)
    }

    fn notify(&mut self, title: String, body: String, severity: Severity, keys: Vec<String>) {
        self.sending.extend(keys.iter().cloned());
        self.notifier.notify(title, body, severity, keys);
    }

    fn mark_alerted(&mut self, keys: impl IntoIterator<Item = String>) {
        let at = now();
        for key in keys {
            self.alerted.insert(key, at);
        }
        let since = at - MAX_CATCHUP;
        self.alerted.retain(|_, notified| *notified >= since);
    }

    pub async fn run(mut self, mut rx: mpsc::Receiver<Msg>) {
        let mut flush = interval(Duration::from_secs_f64(GROUP_WINDOW));
        loop {
            tokio::select! {
                Some(msg) = rx.recv() => self.handle(msg),
                _ = flush.tick() => self.flush_restarts(),
            }
            self.update_caught_up();
        }
    }

    fn update_caught_up(&mut self) {
        let at = now();
        let watching = WATCHES.iter().all(|w| {
            self.watches.get(w).is_some_and(|s| {
                s.listed && s.failed_at.is_none_or(|failed| at - failed > FAILURE_GAP)
            })
        });
        if watching && self.fetching == 0 && self.pending.is_empty() && self.sending.is_empty() {
            self.catching_up = false;
        }
        let caught_up = watching && !self.catching_up;
        if self
            .caught_up
            .send_if_modified(|v| std::mem::replace(v, caught_up) != caught_up)
        {
            info!(
                "heartbeat {}",
                if caught_up {
                    "renewing"
                } else {
                    "paused until caught up"
                }
            );
        }
    }

    fn track<K>(&mut self, watch: Watch, event: &Event<K>) {
        let state = self.watches.entry(watch).or_default();
        match event {
            Event::InitDone => {
                state.listed = true;
                state.failed_at = None;
            }
            Event::Apply(_) | Event::Delete(_) => state.failed_at = None,
            Event::Init | Event::InitApply(_) => {}
        }
    }

    fn handle(&mut self, msg: Msg) {
        match msg {
            Msg::Listing { watch, since } => {
                self.watches.entry(watch).or_default().listed = false;
                self.catching_up = true;
                match watch {
                    Watch::Pods => self.pods_since = since,
                    Watch::Events => self.events_since = since,
                    Watch::Nodes => self.nodes_since = since,
                }
            }
            Msg::Failed { watch } => self.watches.entry(watch).or_default().failed_at = Some(now()),
            Msg::Pod(event) => {
                self.track(Watch::Pods, event.as_ref());
                self.on_pod(&event);
            }
            Msg::Warning(event) => {
                self.track(Watch::Events, event.as_ref());
                self.on_warning_event(&event);
            }
            Msg::Node(event) => {
                self.track(Watch::Nodes, event.as_ref());
                self.on_node_event(&event);
            }
            Msg::Restart(restart) => {
                self.fetching = self.fetching.saturating_sub(1);
                self.pending.push(*restart);
            }
            Msg::Sent { keys, sent } => {
                for key in &keys {
                    self.sending.remove(key);
                }
                if sent {
                    self.mark_alerted(keys);
                }
            }
        }
    }
}
