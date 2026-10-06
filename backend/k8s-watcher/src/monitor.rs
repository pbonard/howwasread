//! Owns every piece of state and decides what is recorded and what alerts.
//!
//! The watch tasks only send messages here, and this task handles them one at a time, so the state needs no lock.

use std::collections::{HashMap, VecDeque};
use std::time::Duration;

use k8s_openapi::api::core::v1::{
    ContainerStateTerminated, ContainerStatus, Event as CoreEvent, Node, Pod,
};
use kube::api::{DynamicObject, LogParams};
use kube::runtime::watcher::Event;
use kube::{Api, Client, ResourceExt};
use serde::Serialize;
use serde_json::json;
use tokio::sync::mpsc;
use tokio::time::{interval, timeout};

use crate::health::{Check, Problems};
use crate::notify::{Notifier, Severity};
use crate::quantity::parse_quantity;
use crate::record::record;

const GROUP_WINDOW: f64 = 30.0;
const CRASH_LOOP_RESTARTS: usize = 3;
const CRASH_LOOP_WINDOW: f64 = 3600.0;
const MEMORY_ALERT_RATIO: f64 = 0.9;
const MEMORY_SAMPLES_KEPT: usize = 4;
const ALERT_EVENT_REASONS: [&str; 7] = [
    "OOMKilling",
    "SystemOOM",
    "Evicted",
    "FailedScheduling",
    "NodeNotReady",
    "FailedMount",
    "FailedAttachVolume",
];
const EVENT_ALERT_COOLDOWN: f64 = 1800.0;
const NODE_BAD: [(&str, &str); 4] = [
    ("Ready", "False"),
    ("MemoryPressure", "True"),
    ("DiskPressure", "True"),
    ("PIDPressure", "True"),
];

/// (namespace, pod, container)
pub type ContainerKey = (String, String, String);

// kubernetes objects are boxed, an enum is as large as its largest variant and the channel holds 1024 of them
pub enum Msg {
    Pod(Box<Event<Pod>>),
    Warning(Box<Event<CoreEvent>>),
    Node(Box<Event<Node>>),
    Resource {
        plural: &'static str,
        check: Check,
        event: Box<Event<DynamicObject>>,
    },
    Memory {
        at: f64,
        samples: Vec<(ContainerKey, f64)>,
    },
    Restart(Box<Restart>),
}

pub struct Settings {
    pub crd_grace: f64,
    pub cluster_restart_min: usize,
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
    memory_before: Option<f64>,
    memory_sampled_seconds_before: Option<i64>,
    last_logs: String,
    #[serde(skip)]
    finished: f64,
}

struct Unhealthy {
    since: f64,
    problem: String,
    alerted: bool,
}

pub struct Monitor {
    client: Client,
    notifier: Notifier,
    settings: Settings,
    // a log fetch runs in its own task and sends the finished restart back through this
    tx: mpsc::Sender<Msg>,
    pods_listed: bool,
    restart_counts: HashMap<ContainerKey, i32>,
    memory: HashMap<ContainerKey, VecDeque<(f64, f64)>>,
    restart_times: HashMap<ContainerKey, VecDeque<f64>>,
    crash_loop_alerted: HashMap<ContainerKey, f64>,
    pending: HashMap<i64, Vec<Restart>>,
    pending_since: HashMap<i64, f64>,
    event_alerted: HashMap<(String, String, String), f64>,
    node_conditions: HashMap<(String, String), String>,
    // (plural, namespace, name, check)
    unhealthy: HashMap<(String, String, String, String), Unhealthy>,
}

pub fn now() -> f64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs_f64()
}

fn mib(bytes: Option<f64>) -> String {
    bytes.map_or("?".into(), |b| format!("{:.0}Mi", b / 1048576.0))
}

impl Monitor {
    pub fn new(
        client: Client,
        notifier: Notifier,
        settings: Settings,
        tx: mpsc::Sender<Msg>,
    ) -> Self {
        Self {
            client,
            notifier,
            settings,
            tx,
            pods_listed: false,
            restart_counts: HashMap::new(),
            memory: HashMap::new(),
            restart_times: HashMap::new(),
            crash_loop_alerted: HashMap::new(),
            pending: HashMap::new(),
            pending_since: HashMap::new(),
            event_alerted: HashMap::new(),
            node_conditions: HashMap::new(),
            unhealthy: HashMap::new(),
        }
    }

    pub async fn run(mut self, mut rx: mpsc::Receiver<Msg>) {
        let mut flush = interval(Duration::from_secs(2));
        let mut grace = interval(Duration::from_secs(10));
        loop {
            tokio::select! {
                Some(msg) = rx.recv() => self.handle(msg),
                _ = flush.tick() => self.flush_restart_groups(),
                _ = grace.tick() => self.check_crd_grace(),
            }
        }
    }

    fn handle(&mut self, msg: Msg) {
        match msg {
            Msg::Pod(event) => self.on_pod(&event),
            Msg::Warning(event) => {
                if let Event::Apply(event) = event.as_ref() {
                    self.on_warning(event);
                }
            }
            Msg::Node(event) => {
                if let Event::Apply(node) | Event::InitApply(node) = event.as_ref() {
                    self.on_node(node);
                }
            }
            Msg::Resource {
                plural,
                check,
                event,
            } => self.on_resource(plural, check, &event),
            Msg::Memory { at, samples } => {
                for (key, bytes) in samples {
                    let kept = self.memory.entry(key).or_default();
                    kept.push_back((at, bytes));
                    if kept.len() > MEMORY_SAMPLES_KEPT {
                        kept.pop_front();
                    }
                }
            }
            Msg::Restart(restart) => {
                let bucket = restart.finished as i64 / 5;
                self.pending.entry(bucket).or_default().push(*restart);
                self.pending_since.entry(bucket).or_insert_with(now);
            }
        }
    }

    // ---- pods: restarts -------------------------------------------------------

    fn on_pod(&mut self, event: &Event<Pod>) {
        match event {
            Event::Init => {}
            Event::InitDone => self.pods_listed = true,
            // the first list only sets the baseline, a relist after a dropped watch still catches restarts in the gap
            Event::InitApply(pod) => self.on_pod_apply(pod, !self.pods_listed),
            Event::Apply(pod) => self.on_pod_apply(pod, false),
            Event::Delete(pod) => {
                for cs in container_statuses(pod) {
                    let key = (
                        pod.namespace().unwrap_or_default(),
                        pod.name_any(),
                        cs.name.clone(),
                    );
                    self.restart_counts.remove(&key);
                    self.memory.remove(&key);
                    self.restart_times.remove(&key);
                    self.crash_loop_alerted.remove(&key);
                }
            }
        }
    }

    fn on_pod_apply(&mut self, pod: &Pod, baseline: bool) {
        for cs in container_statuses(pod) {
            let key = (
                pod.namespace().unwrap_or_default(),
                pod.name_any(),
                cs.name.clone(),
            );
            let previous = self.restart_counts.insert(key.clone(), cs.restart_count);
            let Some(previous) = previous else { continue };
            if baseline || cs.restart_count <= previous {
                continue;
            }
            if let Some(terminated) = cs.last_state.as_ref().and_then(|s| s.terminated.as_ref()) {
                self.on_restart(pod, cs, terminated, key);
            }
        }
    }

    fn on_restart(
        &self,
        pod: &Pod,
        cs: &ContainerStatus,
        terminated: &ContainerStateTerminated,
        key: ContainerKey,
    ) {
        let finished = terminated
            .finished_at
            .as_ref()
            .map_or_else(now, |t| t.0.as_second() as f64);
        let limit = pod
            .spec
            .as_ref()
            .and_then(|spec| spec.containers.iter().find(|c| c.name == cs.name))
            .and_then(|c| c.resources.as_ref()?.limits.as_ref()?.get("memory"))
            .and_then(|q| parse_quantity(&q.0));
        let started = terminated
            .started_at
            .as_ref()
            .map_or(0.0, |t| t.0.as_second() as f64);
        // a sample from while this instance ran, an earlier one belongs to the previous instance and a later one to the
        // next
        let before = self
            .memory
            .get(&key)
            .and_then(|kept| {
                kept.iter()
                    .rev()
                    .find(|(at, _)| (started..=finished).contains(at))
            })
            .copied();
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
            memory_before: before.map(|(_, bytes)| bytes),
            memory_sampled_seconds_before: before.map(|(at, _)| (finished - at) as i64),
            last_logs: String::new(),
            finished,
        };
        let pods: Api<Pod> = Api::namespaced(self.client.clone(), &key.0);
        let params = LogParams {
            container: Some(key.2.clone()),
            previous: true,
            tail_lines: Some(self.settings.log_tail_lines),
            ..Default::default()
        };
        let tx = self.tx.clone();
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

    // restarts that ended in the same few seconds are held together, many of them at once is one host event
    fn flush_restart_groups(&mut self) {
        let now = now();
        // collect first, the map can't be changed while it is being iterated
        let ready: Vec<i64> = self
            .pending_since
            .iter()
            .filter(|(_, since)| now - **since >= GROUP_WINDOW)
            .map(|(b, _)| *b)
            .collect();
        for bucket in ready {
            self.pending_since.remove(&bucket);
            let restarts = self.pending.remove(&bucket).unwrap_or_default();
            if restarts.len() >= self.settings.cluster_restart_min {
                self.on_cluster_restart(restarts);
            } else {
                for restart in restarts {
                    self.classify_restart(restart);
                }
            }
        }
    }

    fn on_cluster_restart(&self, restarts: Vec<Restart>) {
        let mut nodes: Vec<&str> = restarts.iter().filter_map(|r| r.node.as_deref()).collect();
        nodes.sort();
        nodes.dedup();
        let mut codes: Vec<String> = restarts.iter().map(|r| r.exit_code.to_string()).collect();
        codes.sort();
        codes.dedup();
        let mut pods: Vec<String> = restarts
            .iter()
            .map(|r| format!("{}/{}", r.namespace, r.pod))
            .collect();
        pods.sort();
        pods.dedup();
        let at = restarts[0].finished_at.clone().unwrap_or_default();
        for restart in &restarts {
            let mut data = serde_json::to_value(restart).unwrap_or_default();
            data["cause"] = json!("cluster_restart");
            record(
                "restart",
                Severity::Info,
                Some(&restart.namespace),
                Some(&restart.pod),
                data,
            );
        }
        record(
            "cluster_restart",
            Severity::Warning,
            None,
            None,
            json!({"containers": restarts.len(), "nodes": nodes, "exit_codes": codes, "finished_at": at}),
        );
        let more = if pods.len() > 15 {
            format!("\n… and {} more", pods.len() - 15)
        } else {
            String::new()
        };
        self.notifier.notify(
            format!("{} containers restarted together", restarts.len()),
            format!(
                "node {} · exit codes {} · at {at}\nlikely a node or container runtime restart, not an app crash\n{}{more}",
                nodes.join(", "), codes.join(", "), pods.iter().take(15).cloned().collect::<Vec<_>>().join("\n"),
            ),
            Severity::Warning,
        );
    }

    fn classify_restart(&mut self, r: Restart) {
        let key = (r.namespace.clone(), r.pod.clone(), r.container.clone());
        let ratio = r
            .memory_before
            .zip(r.memory_limit)
            .map(|(before, limit)| before / limit);
        let oom = r.reason.as_deref() == Some("OOMKilled")
            || (r.exit_code == 137 && ratio.is_some_and(|ratio| ratio >= MEMORY_ALERT_RATIO));
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
        let mut data = serde_json::to_value(&r).unwrap_or_default();
        data["cause"] = json!(cause);
        data["crash_loop"] = json!(crash_loop);
        record("restart", severity, Some(&r.namespace), Some(&r.pod), data);
        if severity == Severity::Info {
            return;
        }
        let what = match cause {
            "oom" => "out of memory".to_string(),
            "killed" => "killed (SIGKILL)".to_string(),
            _ => format!("exited with {}", r.exit_code),
        };
        let title = format!("{}{what}", if crash_loop { "crash loop: " } else { "" });
        // samples are taken every METRICS_INTERVAL, memory that grows faster than that is not seen
        let sampled = ratio.map_or(String::new(), |ratio| {
            format!(
                " ({:.0}%, sampled {}s before)",
                ratio * 100.0,
                r.memory_sampled_seconds_before.unwrap_or(0)
            )
        });
        let lines: Vec<&str> = r.last_logs.lines().collect();
        let logs = lines[lines.len().saturating_sub(8)..].join("\n");
        let logs: String = logs
            .chars()
            .rev()
            .take(1500)
            .collect::<Vec<_>>()
            .into_iter()
            .rev()
            .collect();
        let in_window = if crash_loop {
            format!(" · {restarts_in_window} restarts in the last hour")
        } else {
            String::new()
        };
        self.notifier.notify(
            format!("{title} · {}/{} [{}]", r.namespace, r.pod, r.container),
            format!(
                "exit {} ({}) · memory {} / {}{sampled} · restart #{}{in_window}\n```\n{logs}\n```",
                r.exit_code,
                r.reason.as_deref().unwrap_or("None"),
                mib(r.memory_before),
                mib(r.memory_limit),
                r.restart_count,
            ),
            severity,
        );
    }

    // ---- warning events -------------------------------------------------------

    fn on_warning(&mut self, event: &CoreEvent) {
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
        record(
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
        if !alert {
            return;
        }
        let key = (namespace.clone(), target.clone(), reason.clone());
        if now() - self.event_alerted.get(&key).copied().unwrap_or(0.0) < EVENT_ALERT_COOLDOWN {
            return;
        }
        self.event_alerted.insert(key, now());
        self.notifier.notify(
            format!("{reason} · {namespace}/{target}"),
            message,
            Severity::Critical,
        );
    }

    // ---- nodes ----------------------------------------------------------------

    fn on_node(&mut self, node: &Node) {
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
            if (previous.is_none() && !bad) || previous.as_deref() == Some(c.status.as_str()) {
                continue;
            }
            let severity = if bad {
                Severity::Critical
            } else {
                Severity::Resolved
            };
            let message = c.message.clone().unwrap_or_default();
            record(
                "node",
                severity,
                None,
                Some(&name),
                json!({"condition": c.type_, "from": previous, "to": c.status, "message": message}),
            );
            self.notifier.notify(
                format!("node {name}: {}={}", c.type_, c.status),
                message,
                severity,
            );
        }
    }

    // ---- operator resources ---------------------------------------------------

    fn on_resource(&mut self, plural: &'static str, check: Check, event: &Event<DynamicObject>) {
        let (object, deleted) = match event {
            Event::Apply(object) | Event::InitApply(object) => (object, false),
            Event::Delete(object) => (object, true),
            Event::Init | Event::InitDone => return,
        };
        let namespace = object.namespace().unwrap_or_default();
        let name = object.name_any();
        let problems = if deleted {
            Problems::new()
        } else {
            check(&object.data)
        };
        let now = now();
        for (check_name, problem) in &problems {
            let key = (
                plural.to_string(),
                namespace.clone(),
                name.clone(),
                check_name.clone(),
            );
            match problem {
                Some(problem) => {
                    self.unhealthy
                        .entry(key)
                        .and_modify(|state| state.problem = problem.clone())
                        .or_insert_with(|| Unhealthy {
                            since: now,
                            problem: problem.clone(),
                            alerted: false,
                        });
                }
                None => {
                    if let Some(state) = self.unhealthy.remove(&key)
                        && state.alerted
                    {
                        self.resolved(&key, now - state.since);
                    }
                }
            }
        }
        self.unhealthy.retain(|(p, ns, n, check_name), _| {
            !(p == plural && *ns == namespace && *n == name && !problems.contains_key(check_name))
        });
    }

    // an operator resource is unhealthy for a moment during every rollout, so only a lasting problem alerts
    fn check_crd_grace(&mut self) {
        let now = now();
        // unhealthy is borrowed mutably while notifier is borrowed shared, which is fine for separate fields
        for ((plural, namespace, name, check_name), state) in self.unhealthy.iter_mut() {
            if state.alerted || now - state.since < self.settings.crd_grace {
                continue;
            }
            state.alerted = true;
            let seconds = (now - state.since) as i64;
            record(
                "resource",
                Severity::Critical,
                Some(namespace),
                Some(&format!("{plural}/{name}")),
                json!({"check": check_name, "problem": state.problem, "for_seconds": seconds}),
            );
            self.notifier.notify(
                format!("{plural}/{name} unhealthy for {seconds}s"),
                format!("{namespace}: {}", state.problem),
                Severity::Critical,
            );
        }
    }

    fn resolved(&self, key: &(String, String, String, String), duration: f64) {
        let (plural, namespace, name, check_name) = key;
        let seconds = duration as i64;
        record(
            "resource",
            Severity::Resolved,
            Some(namespace),
            Some(&format!("{plural}/{name}")),
            json!({"check": check_name, "after_seconds": seconds}),
        );
        self.notifier.notify(
            format!("{plural}/{name} recovered"),
            format!("{namespace}: {check_name} is healthy after {seconds}s"),
            Severity::Resolved,
        );
    }
}

fn container_statuses(pod: &Pod) -> impl Iterator<Item = &ContainerStatus> {
    pod.status
        .as_ref()
        .and_then(|s| s.container_statuses.as_ref())
        .into_iter()
        .flatten()
}
