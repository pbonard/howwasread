use crate::monitor::ContainerKey;
use crate::notify::Severity;
use k8s_openapi::api::core::v1::{ContainerStatus, Pod};
use k8s_openapi::apimachinery::pkg::apis::meta::v1::Time;
use k8s_openapi::jiff::Timestamp;
use kube::ResourceExt;
use serde_json::{Value, json};
use std::str::FromStr;

const UNITS: [(&str, f64); 9] = [
    ("Ki", 1024.0),
    ("Mi", 1048576.0),
    ("Gi", 1073741824.0),
    ("Ti", 1099511627776.0),
    ("k", 1e3),
    ("M", 1e6),
    ("G", 1e9),
    ("T", 1e12),
    ("m", 1e-3),
];

pub fn parse_quantity(quantity: &str) -> Option<f64> {
    for (unit, factor) in UNITS {
        if let Some(number) = quantity.strip_suffix(unit) {
            return number.parse::<f64>().ok().map(|n| n * factor);
        }
    }
    quantity.parse().ok()
}

pub fn log(
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

pub fn env_or<T: FromStr>(name: &str, default: T) -> T {
    std::env::var(name)
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(default)
}

pub fn truncate(text: &str, max_chars: usize) -> String {
    text.chars().take(max_chars).collect()
}

pub fn now() -> f64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs_f64()
}

/// Whether `time` is at or after `since`, a missing time never is.
pub fn happened_after(time: Option<&Time>, since: f64) -> bool {
    time.is_some_and(|t| t.0.as_second() as f64 >= since)
}

pub fn mib(bytes: Option<f64>) -> String {
    bytes.map_or("?".into(), |b| format!("{:.0}Mi", b / 1048576.0))
}

pub fn container_key(pod: &Pod, cs: &ContainerStatus) -> ContainerKey {
    (
        pod.namespace().unwrap_or_default(),
        pod.name_any(),
        cs.name.clone(),
    )
}

pub fn container_statuses(pod: &Pod) -> impl Iterator<Item = &ContainerStatus> {
    pod.status
        .as_ref()
        .and_then(|s| s.container_statuses.as_ref())
        .into_iter()
        .flatten()
}
