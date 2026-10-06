# k8s-watcher

Watches the cluster and records every problem, alerting only on the ones that need attention.

| Watches | Records | Alerts |
|---|---|---|
| Pods | every container restart, with exit code, memory before it ended, limit, last log lines | out of memory, SIGKILL (137), 3+ restarts in an hour |
| Pods | many containers ending in the same seconds, as one incident | one message for the node or runtime restart |
| Warning events | all of them | OOMKilling, Evicted, FailedScheduling, NodeNotReady, FailedMount, … (once per 30 min) |
| Nodes | Ready / memory, disk, PID pressure changes | when it goes bad and when it recovers |
| VitessShard, FlinkDeployment, Kafka | health changes | unhealthy for longer than `CRD_GRACE_SECONDS`, and recovery |

Every record is one JSON line on stdout. Alloy ships the logs of pods labeled `framework: none` (the helm default)
to Grafana Cloud Loki, so the history is kept and searched there.

## Run locally

A member of the Rust workspace in `backend/` (`backend/Cargo.toml`, next to `go.mod`).

```sh
cd backend
cargo run -p k8s-watcher        # uses your current kubeconfig
cargo test -p k8s-watcher
```

Tests live in `tests/`, like `test/` in the go services: each file is compiled as its own crate against the public
API of `src/lib.rs`, `src/main.rs` only starts the watcher.

Everything goes to stdout as JSON lines, the logs (`level`, `fields.message`) and the records (`kind`).

## Alerts

Without any of these, alerts are only logged.

| Env | |
|---|---|
| `DISCORD_WEBHOOK_URL` | Discord channel webhook |
| `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID` | Telegram bot |

In the cluster they come from the optional secret `k8s-watcher-notify`, keep it as a SealedSecret like the others.

## Settings

| Env | Default | |
|---|---|---|
| `METRICS_INTERVAL` | `30` | seconds between memory samples, memory growing faster than this is not seen |
| `CRD_GRACE_SECONDS` | `120` | how long an operator resource may be unhealthy before alerting |
| `CLUSTER_RESTART_MIN` | `5` | containers ending together that count as one node or runtime restart |
| `LOG_TAIL_LINES` | `30` | previous log lines kept per restart |

## Deploy

Like the other backends: a push to `main` that changes `backend/k8s-watcher/` builds the image and writes its tag into
`infra/k8s/helm/values.yaml`, and Argo CD syncs it to the `backend` namespace. `watcher: true` in values adds the
service account, the read only cluster role and the data volume (`templates/watchers.yaml`).

## Query the history

In Grafana (Explore → the Grafana Cloud Loki data source):

```logql
{namespace="backend", container="k8s-watcher"} | json | kind="restart"
```

```logql
sum by (name) (count_over_time({namespace="backend", container="k8s-watcher"} | json | kind="restart" [7d]))
```

Without Grafana, `kubectl logs -n backend deploy/k8s-watcher` shows the records and logs since the last restart of the
watcher.

## Known limits

- This cluster reports a container killed for using too much memory as `exit 137, reason Error`, not `OOMKilled`.
  It is then classified as out of memory only when the last sample was at least 90% of the limit.
- Containers that live shorter than one metrics-server scrape have no memory sample, only samples taken while the
  ended instance ran are used.
- Warning events that happen while the watch is reconnecting are not recorded, restarts are, through the relist.
- History is as long as the Grafana Cloud log retention, and records written while Alloy can't reach Grafana Cloud
  may be lost.
