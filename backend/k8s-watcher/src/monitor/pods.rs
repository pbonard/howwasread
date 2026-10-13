use std::collections::HashSet;
use std::time::Duration;

use k8s_openapi::api::core::v1::{ContainerStateTerminated, ContainerStatus, Pod};
use kube::api::LogParams;
use kube::runtime::watcher::Event;
use kube::{Api, ResourceExt};
use serde_json::json;
use tokio::time::timeout;

use super::{ContainerKey, Monitor, Msg, Restart};
use crate::notify::Severity;
use crate::util::{
    container_key, container_statuses, happened_after, log, mib, now, parse_quantity,
};

const MAX_RESTART_LINES: usize = 30;
const CRASH_LOOP_RESTARTS: usize = 3;
const CRASH_LOOP_WINDOW: f64 = 3600.0;

impl Monitor {
    pub(super) fn on_pod(&mut self, event: &Event<Pod>) {
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

    pub(super) fn flush_restarts(&mut self) {
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
}
