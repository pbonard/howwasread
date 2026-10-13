use std::collections::{HashMap, HashSet, VecDeque};
use std::time::Duration;

use k8s_openapi::api::core::v1::{
    ContainerStateTerminated, ContainerStatus, Event as CoreEvent, Node, Pod,
};
use kube::api::LogParams;
use kube::runtime::watcher::Event;
use kube::{Api, Client, ResourceExt};
use serde::Serialize;
use serde_json::json;
use tokio::sync::{mpsc, watch};
use tokio::time::{interval, timeout};
use tracing::info;

use crate::notify::{Notifier, Severity};
use crate::util::{
    container_key, container_statuses, happened_after, log, mib, now, parse_quantity,
};

const GROUP_WINDOW: f64 = 30.0;
pub const HEARTBEAT_SECONDS: u64 = 10;
pub const MAX_CATCHUP: f64 = 86400.0;
pub const FAILURE_GAP: f64 = 120.0;
const MAX_RESTART_LINES: usize = 30;
const CRASH_LOOP_RESTARTS: usize = 3;
const CRASH_LOOP_WINDOW: f64 = 3600.0;
const ALERT_EVENT_REASONS: [&str; 7] = [
    "OOMKilling",
    "SystemOOM",
    "Evicted",
    "FailedScheduling",
    "NodeNotReady",
    "FailedMount",
    "FailedAttachVolume",
];
const NODE_BAD: [(&str, &str); 4] = [
    ("Ready", "False"),
    ("MemoryPressure", "True"),
    ("DiskPressure", "True"),
    ("PIDPressure", "True"),
];

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
    Listing {
        watch: Watch,
        since: f64,
    },
    Failed {
        watch: Watch,
    },
    Pod(Box<Event<Pod>>),
    Warning(Box<Event<CoreEvent>>),
    Node(Box<Event<Node>>),
    Restart(Box<Restart>),
    Sent {
        keys: Vec<String>,
        sent: bool,
    },
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

    fn on_warning_event(&mut self, event: &Event<CoreEvent>) {
        match event {
            Event::Apply(event) => self.on_warning(event, None),
            Event::InitApply(event) => self.on_warning(event, Some(self.events_since)),
            Event::Init | Event::InitDone | Event::Delete(_) => {}
        }
    }

    fn on_node_event(&mut self, event: &Event<Node>) {
        match event {
            Event::Apply(node) | Event::InitApply(node) => self.on_node(node, self.nodes_since),
            Event::Init | Event::InitDone | Event::Delete(_) => {}
        }
    }

    fn on_pod(&mut self, event: &Event<Pod>) {
        match event {
            Event::Init => self.listed = Some(HashSet::new()),
            Event::InitApply(pod) => {
                if let Some(listed) = self.listed.as_mut() {
                    listed.extend(container_statuses(pod).map(|cs| container_key(pod, cs)));
                }
                self.on_pod_apply(pod, self.pods_since);
            }
            // a pod deleted while the watch was reconnecting is only missing from the new list, no Delete arrives for it
            Event::InitDone => {
                if let Some(listed) = self.listed.take() {
                    self.restart_counts.retain(|key, _| listed.contains(key));
                    self.restart_times.retain(|key, _| listed.contains(key));
                    self.crash_loop_alerted
                        .retain(|key, _| listed.contains(key));
                }
            }
            Event::Apply(pod) => self.on_pod_apply(pod, self.pods_since),
            Event::Delete(pod) => {
                for cs in container_statuses(pod) {
                    let key = container_key(pod, cs);
                    self.restart_counts.remove(&key);
                    self.restart_times.remove(&key);
                    self.crash_loop_alerted.remove(&key);
                }
            }
        }
    }

    fn on_pod_apply(&mut self, pod: &Pod, since: f64) {
        for cs in container_statuses(pod) {
            let key = container_key(pod, cs);
            let previous = self.restart_counts.insert(key.clone(), cs.restart_count);
            let terminated = cs.last_state.as_ref().and_then(|s| s.terminated.as_ref());
            let restarted = match previous {
                Some(previous) => cs.restart_count > previous,
                // first sight: only a restart that ended while the watcher was away, older ones are history
                None => {
                    cs.restart_count > 0
                        && happened_after(terminated.and_then(|t| t.finished_at.as_ref()), since)
                }
            };
            if !restarted {
                continue;
            }
            let alert_key = format!(
                "{}/{}/{}",
                pod.uid().unwrap_or_default(),
                cs.name,
                cs.restart_count
            );
            if self.is_alerted(&alert_key) {
                continue;
            }
            if let Some(terminated) = terminated {
                self.on_restart(pod, cs, terminated, key, alert_key);
            }
        }
    }

    fn on_restart(
        &mut self,
        pod: &Pod,
        cs: &ContainerStatus,
        terminated: &ContainerStateTerminated,
        key: ContainerKey,
        alert_key: String,
    ) {
        let limit = pod
            .spec
            .as_ref()
            .and_then(|spec| spec.containers.iter().find(|c| c.name == cs.name))
            .and_then(|c| c.resources.as_ref()?.limits.as_ref()?.get("memory"))
            .and_then(|q| parse_quantity(&q.0));
        let mut restart = Restart {
            namespace: key.0.clone(),
            pod: key.1.clone(),
            container: key.2.clone(),
            node: pod.spec.as_ref().and_then(|s| s.node_name.clone()),
            exit_code: terminated.exit_code,
            reason: terminated.reason.clone(),
            started_at: terminated.started_at.as_ref().map(|t| t.0.to_string()),
            finished_at: terminated.finished_at.as_ref().map(|t| t.0.to_string()),
            restart_count: cs.restart_count,
            memory_limit: limit,
            last_logs: String::new(),
            alert_key,
        };
        let pods: Api<Pod> = Api::namespaced(self.client.clone(), &key.0);
        let params = LogParams {
            container: Some(key.2.clone()),
            previous: true,
            tail_lines: Some(self.settings.log_tail_lines),
            ..Default::default()
        };
        let tx = self.tx.clone();
        self.fetching += 1;
        tokio::spawn(async move {
            restart.last_logs =
                match timeout(Duration::from_secs(5), pods.logs(&key.1, &params)).await {
                    Ok(Ok(logs)) => logs,
                    Ok(Err(e)) => format!("(previous logs unavailable: {e})"),
                    Err(_) => "(previous logs unavailable: timed out)".into(),
                };
            let _ = tx.send(Msg::Restart(Box::new(restart))).await;
        });
    }

    fn flush_restarts(&mut self) {
        let restarts = std::mem::take(&mut self.pending);
        let mut severity = Severity::Warning;
        let mut lines = Vec::new();
        let mut keys = Vec::new();
        for restart in restarts {
            let Some((line_severity, line)) = self.classify_restart(&restart) else {
                continue;
            };
            if line_severity == Severity::Critical {
                severity = Severity::Critical;
            }
            lines.push(line);
            keys.push(restart.alert_key);
        }
        if lines.is_empty() {
            return;
        }
        let more = if lines.len() > MAX_RESTART_LINES {
            format!("\n… and {} more", lines.len() - MAX_RESTART_LINES)
        } else {
            String::new()
        };
        self.notify(
            format!(
                "{} restart{}",
                lines.len(),
                if lines.len() == 1 { "" } else { "s" }
            ),
            format!(
                "{}{more}",
                lines[..lines.len().min(MAX_RESTART_LINES)].join("\n")
            ),
            severity,
            keys,
        );
    }

    fn classify_restart(&mut self, r: &Restart) -> Option<(Severity, String)> {
        let key = (r.namespace.clone(), r.pod.clone(), r.container.clone());
        let oom = r.reason.as_deref() == Some("OOMKilled");
        let now = now();
        let times = self.restart_times.entry(key.clone()).or_default();
        times.push_back(now);
        while times.front().is_some_and(|t| now - t > CRASH_LOOP_WINDOW) {
            times.pop_front();
        }
        let restarts_in_window = times.len();
        let crash_loop = restarts_in_window >= CRASH_LOOP_RESTARTS
            && now - self.crash_loop_alerted.get(&key).copied().unwrap_or(0.0) > CRASH_LOOP_WINDOW;
        if crash_loop {
            self.crash_loop_alerted.insert(key, now);
        }
        let cause = match (oom, r.exit_code) {
            (true, _) => "oom",
            (false, 137) => "killed",
            (false, 0) => "exited",
            _ => "error",
        };
        let severity = if oom || crash_loop {
            Severity::Critical
        } else if r.exit_code == 137 {
            Severity::Warning
        } else {
            Severity::Info
        };
        let mut data = serde_json::to_value(r).unwrap_or_default();
        data["cause"] = json!(cause);
        data["crash_loop"] = json!(crash_loop);
        log("restart", severity, Some(&r.namespace), Some(&r.pod), data);
        if severity == Severity::Info {
            return None;
        }
        let what = match cause {
            "oom" => format!("out of memory (limit {})", mib(r.memory_limit)),
            "killed" => "killed (SIGKILL)".to_string(),
            _ => format!("exited with {}", r.exit_code),
        };
        let crash_loop = if crash_loop {
            format!("crash loop, {restarts_in_window} restarts in the last hour · ")
        } else {
            String::new()
        };
        Some((
            severity,
            format!(
                "- {}/{} [{}] · {crash_loop}{what} · restart #{}",
                r.namespace, r.pod, r.container, r.restart_count
            ),
        ))
    }

    fn on_warning(&mut self, event: &CoreEvent, listed_since: Option<f64>) {
        let new = match listed_since {
            Some(since) => happened_after(event.metadata.creation_timestamp.as_ref(), since),
            None => event.series.is_none() && event.count.unwrap_or(1) <= 1,
        };
        if listed_since.is_some() && !new {
            return;
        }
        let object = &event.involved_object;
        let namespace = object
            .namespace
            .clone()
            .or_else(|| event.metadata.namespace.clone())
            .unwrap_or_default();
        let target = format!(
            "{}/{}",
            object.kind.as_deref().unwrap_or(""),
            object.name.as_deref().unwrap_or("")
        );
        let reason = event.reason.clone().unwrap_or_default();
        let message = event.message.clone().unwrap_or_default();
        let alert = ALERT_EVENT_REASONS.contains(&reason.as_str());
        log(
            "event",
            if alert {
                Severity::Warning
            } else {
                Severity::Info
            },
            Some(&namespace),
            Some(&target),
            json!({"reason": reason, "message": message, "count": event.count}),
        );
        let alert_key = event.metadata.uid.clone().unwrap_or_default();
        if !alert || !new || self.is_alerted(&alert_key) {
            return;
        }
        self.notify(
            format!("{reason} · {namespace}/{target}"),
            message,
            Severity::Critical,
            vec![alert_key],
        );
    }

    fn on_node(&mut self, node: &Node, since: f64) {
        let name = node.name_any();
        let conditions = node.status.as_ref().and_then(|s| s.conditions.as_ref());
        for c in conditions.into_iter().flatten() {
            let Some((_, bad_status)) = NODE_BAD.iter().find(|(kind, _)| *kind == c.type_) else {
                continue;
            };
            let previous = self
                .node_conditions
                .insert((name.clone(), c.type_.clone()), c.status.clone());
            let bad = c.status == *bad_status;
            let changed = match previous.as_deref() {
                Some(previous) => previous != c.status,
                None => bad && happened_after(c.last_transition_time.as_ref(), since),
            };
            let alert_key = format!(
                "{name}/{}/{}/{}",
                c.type_,
                c.status,
                c.last_transition_time
                    .as_ref()
                    .map(|t| t.0.to_string())
                    .unwrap_or_default()
            );
            if !changed || self.is_alerted(&alert_key) {
                continue;
            }
            let severity = if bad {
                Severity::Critical
            } else {
                Severity::Resolved
            };
            let message = c.message.clone().unwrap_or_default();
            log(
                "node",
                severity,
                None,
                Some(&name),
                json!({"condition": c.type_, "from": previous, "to": c.status, "message": message}),
            );
            self.notify(
                format!("node {name}: {}={}", c.type_, c.status),
                message,
                severity,
                vec![alert_key],
            );
        }
    }
}
