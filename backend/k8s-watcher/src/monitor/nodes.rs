use k8s_openapi::api::core::v1::Node;
use kube::ResourceExt;
use kube::runtime::watcher::Event;
use serde_json::json;

use super::Monitor;
use crate::notify::Severity;
use crate::util::{happened_after, log};

const NODE_BAD: [(&str, &str); 4] = [
    ("Ready", "False"),
    ("MemoryPressure", "True"),
    ("DiskPressure", "True"),
    ("PIDPressure", "True"),
];

impl Monitor {
    pub(super) fn on_node_event(&mut self, event: &Event<Node>) {
        match event {
            Event::Apply(node) | Event::InitApply(node) => self.on_node(node, self.nodes_since),
            Event::Init | Event::InitDone | Event::Delete(_) => {}
        }
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
