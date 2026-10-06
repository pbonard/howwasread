//! Records every finding as one JSON line on stdout, Alloy ships it to Grafana Cloud Loki with the other pod logs.

use k8s_openapi::jiff::Timestamp;
use serde_json::{Value, json};

use crate::notify::Severity;

pub fn record(
    kind: &str,
    severity: Severity,
    namespace: Option<&str>,
    name: Option<&str>,
    data: Value,
) {
    let time = Timestamp::from_second(Timestamp::now().as_second())
        .expect("now is a valid time")
        .to_string();
    let mut line = json!({
        "time": time, "kind": kind, "severity": severity.as_str(), "namespace": namespace, "name": name,
    });
    if let (Some(line), Some(data)) = (line.as_object_mut(), data.as_object()) {
        line.extend(data.clone());
    }
    println!("{line}");
}
