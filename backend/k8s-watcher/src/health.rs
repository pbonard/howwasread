//! Health checks for the operator custom resources, each returns {check name: problem or None}.

use std::collections::BTreeMap;

use serde_json::Value;

pub type Problems = BTreeMap<String, Option<String>>;
pub type Check = fn(&Value) -> Problems;

pub struct Watched {
    pub group: &'static str,
    pub version: &'static str,
    pub kind: &'static str,
    pub plural: &'static str,
    pub check: Check,
}

pub const RESOURCES: [Watched; 3] = [
    Watched {
        group: "planetscale.com",
        version: "v2",
        kind: "VitessShard",
        plural: "vitessshards",
        check: vitess_shard,
    },
    Watched {
        group: "flink.apache.org",
        version: "v1beta1",
        kind: "FlinkDeployment",
        plural: "flinkdeployments",
        check: flink_deployment,
    },
    Watched {
        group: "kafka.strimzi.io",
        version: "v1",
        kind: "Kafka",
        plural: "kafkas",
        check: kafka,
    },
];

// a missing field reads as Value::Null, so indexing never panics and this returns ""
fn text(value: &Value) -> &str {
    value.as_str().unwrap_or("")
}

fn conditions(data: &Value, healthy: &[(&str, &str)]) -> Problems {
    let found = data["status"]["conditions"]
        .as_array()
        .cloned()
        .unwrap_or_default();
    let mut problems = Problems::new();
    for (kind, want) in healthy {
        let Some(c) = found.iter().find(|c| c["type"] == *kind) else {
            continue;
        };
        let problem = (c["status"] != *want).then(|| {
            format!(
                "{kind}={} {} {}",
                text(&c["status"]),
                text(&c["reason"]),
                text(&c["message"])
            )
            .trim()
            .to_string()
        });
        problems.insert(kind.to_string(), problem);
    }
    problems
}

pub fn vitess_shard(data: &Value) -> Problems {
    let status = &data["status"];
    let mut problems = Problems::from([
        (
            "servingWrites".into(),
            (status["servingWrites"] != "True").then(|| "shard is not serving writes".into()),
        ),
        (
            "hasMaster".into(),
            (status["hasMaster"] != "True").then(|| "shard has no primary".into()),
        ),
    ]);
    for (alias, tablet) in status["tablets"].as_object().into_iter().flatten() {
        let problem = (tablet["available"] != "True").then(|| {
            format!(
                "tablet {alias} ({}) is not available",
                text(&tablet["type"])
            )
        });
        problems.insert(format!("tablet {alias}"), problem);
    }
    problems
}

pub fn flink_deployment(data: &Value) -> Problems {
    let status = &data["status"];
    let state = text(&status["jobStatus"]["state"]);
    let error: String = text(&status["error"]).chars().take(480).collect();
    Problems::from([
        (
            "job".into(),
            (state != "RUNNING").then(|| {
                format!(
                    "job state is {}",
                    if state.is_empty() { "unknown" } else { state }
                )
            }),
        ),
        (
            "lifecycle".into(),
            (status["lifecycleState"] == "FAILED").then(|| format!("lifecycle is FAILED: {error}")),
        ),
    ])
}

pub fn kafka(data: &Value) -> Problems {
    conditions(data, &[("Ready", "True")])
}
