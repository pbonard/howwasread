# Roadmap: conversation history

Status: **planned**

## Design decisions

| Decision | Choice | Reason |
|---|---|---|
| Source | Consume the existing `conversation-cdc` topic | It already carries full rows, delete tombstones, the binlog time as the record timestamp, and a `taskType` header. The Flink job needs no change. |
| Consumer | One consumer group `conversation_history` in **onlineconversation**, covering online and offline | One read of the topic. Offline's history columns must be kept in sync by hand (see Risks). |
| Storage | New Vitess keyspace `history`, 1 shard, `binary_md5` on `conversation_id` | Keeps history growth and load away from `conversation`. All versions of a conversation stay on one shard. A later `Reshard` needs no application change. |
| Version key | `(conversation_id, version = updated_at)`; `1970-01-01 00:00:00` = never edited | `updated_at` only changes on content edits, so registrations don't create versions and a CDC snapshot replay is idempotent. The app already returns `updatedAt`, so reports can send it as `version`. |
| Delete marking | Only the **latest** version gets `deleted_at` | Upserts set `deleted_at = NULL` safely, because records are ordered per conversation (keyed by id). |
| Recovery | Re-insert the row with `updated_at = UTC_TIMESTAMP()` | The recovered conversation becomes a new version, so earlier deletion times are kept. |
| Members | One event-log table, one row per in/out | Rebuild membership at any moment, e.g. just before a deletion. Replays are idempotent. |
| Orphan cleanup | App-side delete of member rows in the conversation delete transaction | MySQL 8.0 FK cascades are not written to the binlog, so CDC wouldn't see them. Vitess-managed FKs are an alternative but not needed. |

## Prerequisites

- [ ] Report flow exists (report endpoints, `report` topic, Python Detoxify worker, `report` table).
- [ ] S3 checkpoints and `upgradeMode: last-state` for `conversation-cdc-job`. Without them, edits made while the job was down are lost from history.
- [ ] Node memory headroom for one more vttablet + mysqld (about 1Gi of limits).

## Phase 1: infrastructure

- [ ] `infra/k8s/bootstrap/vitess-cluster.yaml`: add keyspace `history`, a copy of the `conversation` block:
  - `durabilityPolicy: none`, `equal.parts: 1`
  - 1 replica tablet, same resources, 5Gi volume, same `vitess-init-script`
- [ ] Init script: add `CREATE DATABASE IF NOT EXISTS history;`. First check how the conversation tablet's DB name is set up.
- [ ] New `infra/k8s/bootstrap/history-schema-job.yaml`: a copy of `conversation-schema-job.yaml` targeting keyspace `history`, with the declarative ApplySchema and ApplyVSchema. Add it to `bootstrap.sh` if the conversation job is there.
- [ ] `infra/k8s/bootstrap/kafka-topics.yaml`: set `conversation-cdc` `config.retention.ms: 604800000` (7 days; the broker default is 48h). The history consumer can't be down longer than retention without losing versions.

### Schema (`history` keyspace)

```sql
CREATE TABLE online_conversation_history
(
    conversation_id BINARY(16)  NOT NULL,
    version         DATETIME    NOT NULL,  -- updated_at; '1970-01-01 00:00:00' = original
    novel           TEXT,
    short_story     TEXT,
    poem            TEXT,
    play            TEXT,
    film            TEXT,
    written_by      TEXT        NOT NULL,
    rule            TEXT,
    capacity        INT         NOT NULL,
    time            DATETIME    NOT NULL,
    length_minutes  INT         NOT NULL,
    deleted_at      DATETIME(3) NULL,      -- set on the latest version by the delete tombstone
    PRIMARY KEY (conversation_id, version)
);

CREATE TABLE offline_conversation_history
(
    conversation_id BINARY(16)  NOT NULL,
    version         DATETIME    NOT NULL,
    novel           TEXT,
    poem            TEXT,
    short_story     TEXT,
    play            TEXT,
    film            TEXT,
    written_by      TEXT        NOT NULL,
    rule            TEXT,
    time            DATETIME    NOT NULL,
    length_minutes  INT         NOT NULL,
    maps_link       TEXT        NOT NULL,
    location        TEXT,
    latitude        DOUBLE      NOT NULL,  -- geo columns kept so recovery can rebuild the full row
    longitude       DOUBLE      NOT NULL,
    city            TEXT,
    h3_res5         VARCHAR(15) NOT NULL,
    h3_res7         VARCHAR(15) NOT NULL,
    deleted_at      DATETIME(3) NULL,
    PRIMARY KEY (conversation_id, version)
);

CREATE TABLE conversation_member_history
(
    conversation_id BINARY(16)  NOT NULL,
    member_type     VARCHAR(40) NOT NULL,  -- CDC taskType header, e.g. 'online_conversation_registrant'
    member_id       BINARY(16)  NOT NULL,
    changed_at      DATETIME(3) NOT NULL,  -- kafka record timestamp (binlog time)
    action          VARCHAR(3)  NOT NULL,  -- 'in' | 'out'
    PRIMARY KEY (conversation_id, member_type, member_id, changed_at, action)
);
```

The `vschema.json` has `"sharded": true`, vindex `binary_md5`, and all three tables on `conversation_id`.

`current_registrants` is not stored. On recovery, recompute it from the restored registrants.

## Phase 2: orphan cleanup

Deleting a conversation leaves its member rows behind today.

- [ ] onlineconversation: add `DeleteMembers(ctx, session, conversationId)` to the repository. It deletes from `online_conversation_moderator`, `_registrant`, `_ban` and `_notification`. Call it in `service.DeleteConversation` after `DeleteOnlineConversationIfModerator` succeeds, inside the same transaction.
- [ ] offlineconversation: in `OfflineConversationService.delete`, after `deleteIfModerator` returns 1, delete the moderator and participant rows by conversation id. Use `@Modifying @Query` methods, in the existing transaction.
- [ ] Optional: make offline `join()` check that the conversation exists. It currently uses `getReferenceById`, so a join racing a delete can create a new orphan.
- [ ] Tests: the delete calls `DeleteMembers` inside the transaction, and a non-moderator delete doesn't.

## Phase 3: history consumer (onlineconversation)

- [ ] **DB handle**: a second `*sql.DB` to vtgate with `DBName: "history"`, created in `NewRepository` and pinged at startup, plus `BeginHistoryTx`. There are no cross-keyspace transactions.
- [ ] **Repository** (`internal/repository/history.go`):
  - `UpsertOnlineConversationHistory` / `UpsertOfflineConversationHistory`: `INSERT ... ON DUPLICATE KEY UPDATE <content>, deleted_at = NULL`
  - `FindLatestHistoryVersion`: `SELECT MAX(version) WHERE conversation_id = ?`
  - `MarkHistoryDeleted`: `UPDATE ... SET deleted_at = ? WHERE conversation_id = ? AND version = ?`
  - `InsertMemberHistory`: `INSERT IGNORE`
  - Choose the table with a typed const, never caller-supplied SQL.
- [ ] **Service** (`internal/service/history.go`):
  - `SaveConversationHistory`: the version is the parsed `updated_at` (`yyyy-MM-ddTHH:mm:ssZ`), or epoch 0 when null
  - `MarkConversationDeleted`: in one history transaction, find the latest version (skip if none), then mark it deleted
  - `SaveMemberHistory`
- [ ] **CDC structs** (`internal/dto`), snake_case JSON:
  - online and offline conversation rows
  - the member key `{conversation_id, member_id}` (member keys are JSON; conversation keys are raw uuid strings)
- [ ] **Consumer** (`internal/consumer/root.go`, modeled on `messagepreprocess/internal/consumer`):
  - group `conversation_history`, topic `conversation-cdc`, `OffsetOldest`
  - switch on the `taskType` header before parsing, and skip unknown types
  - an empty value is a tombstone:
    - conversation types → `MarkConversationDeleted` using the record timestamp
    - member types → `SaveMemberHistory(..., "out")`
  - otherwise upsert, or `"in"` for member types
  - DB errors: retry with capped backoff and don't mark the offset until success, so per-conversation order holds. **Don't** use `exponential-backoff-retry`: it re-publishes to `conversation-cdc` and search would consume it again.
  - malformed payload: log and mark
- [ ] **Wiring** (`internal/root.go`): run the consumer in a goroutine. It handles SIGTERM itself, so call `g.GracefulStop()` when it returns.
- [ ] `go tool mockery`, then tests in `onlineconversation/test`:
  - version from `updated_at`, and epoch 0 when null
  - delete marks the latest version, and does nothing when there's no history
  - member in/out fields pass through

## Phase 4: use it

- [ ] **Report validation**: the report carries `version`. Look it up:
  ```sql
  SELECT * FROM online_conversation_history WHERE conversation_id = ? AND version = ?;
  ```
  If there's no match yet, retry briefly, since CDC lags slightly behind.
- [ ] **Recovery**:
  1. Take the latest version's content.
  2. Rebuild the members: for each `(member_type, member_id)`, take the last event at or before the deletion time; members whose last event is `in` are restored. The deletion's own `out` rows have the same time as `deleted_at`, so either compare at second precision or treat `out` rows at `deleted_at` as "was in".
  3. Re-insert the conversation with `updated_at = UTC_TIMESTAMP()`, then the members, and recompute `current_registrants`.
  4. Decide whether to reschedule reminders for restored notification subscribers. The original reminders were cancelled at deletion.

## Verification

- `go build ./... && go vet ./... && go test -race ./...` in `backend/`; `./gradlew test` in offlineconversation
- Deploy:
  1. `vitess-cluster.yaml`, then wait for the `history` tablet to be serving
  2. `history-schema-job.yaml` and `kafka-topics.yaml`
  3. redeploy onlineconversation and offlineconversation
- In the cluster:
  - create, edit and delete an online conversation
  - `online_conversation_history` has 2 versions, the latest with `deleted_at`
  - `conversation_member_history` shows the moderator's `in` then `out`
  - `conversation_history` consumer lag is 0

## Risks / notes

- **Offline schema drift**: onlineconversation writes `offline_conversation_history`. When `offline_conversation` gains a column, update the Go consumer too. Note this on the table.
- **`updated_at` is only precise to the second**: two edits within one second share a version, and the later one wins. Switch to `DATETIME(3)` if that matters.
- **History starts when the consumer starts**: conversations that already exist get one starting version, from what's still in the topic or from the next CDC snapshot.
- **Retention and checkpoints**: gaps appear if the consumer is down longer than topic retention, or if the CDC job restarts without a checkpoint.
