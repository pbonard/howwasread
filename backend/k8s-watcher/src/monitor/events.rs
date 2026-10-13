use k8s_openapi::api::core::v1::Event as CoreEvent;
use kube::runtime::watcher::Event;
use serde_json::json;

use super::Monitor;
use crate::notify::Severity;
use crate::util::{happened_after, log};

const ALERT_EVENT_REASONS: [&str; 7] = [
    "OOMKilling",
    "SystemOOM",
    "Evicted",
    "FailedScheduling",
    "NodeNotReady",
    "FailedMount",
    "FailedAttachVolume",
];

impl Monitor {
    pub(super) fn on_warning_event(&mut self, event: &Event<CoreEvent>) {
        match event {
            Event::Apply(event) => self.on_warning(event, None),
            Event::InitApply(event) => self.on_warning(event, Some(self.events_since)),
            Event::Init | Event::InitDone | Event::Delete(_) => {}
        }
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
}
