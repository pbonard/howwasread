use k8s_watcher::health::{flink_deployment, kafka, vitess_shard};
use serde_json::json;

#[test]
fn vitess_shard_reports_an_unavailable_tablet() {
    let data = json!({"status": {"servingWrites": "True", "hasMaster": "True",
        "tablets": {"zone1-1": {"type": "primary", "available": "False"}}}});
    let problems = vitess_shard(&data);
    assert_eq!(problems["servingWrites"], None);
    assert_eq!(
        problems["tablet zone1-1"].as_deref(),
        Some("tablet zone1-1 (primary) is not available")
    );
}

#[test]
fn kafka_is_healthy_when_ready_and_skips_a_missing_condition() {
    assert_eq!(
        kafka(&json!({"status": {"conditions": [{"type": "Ready", "status": "True"}]}}))["Ready"],
        None
    );
    assert!(kafka(&json!({"status": {}})).is_empty());
}

#[test]
fn flink_job_that_is_not_running_is_a_problem() {
    let problems = flink_deployment(
        &json!({"status": {"jobStatus": {"state": "FAILED"}, "lifecycleState": "STABLE"}}),
    );
    assert_eq!(problems["job"].as_deref(), Some("job state is FAILED"));
    assert_eq!(problems["lifecycle"], None);
}
