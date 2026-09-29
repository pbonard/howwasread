package repository

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

// InsertOutbox stores an empty taskType as NULL, so the CDC job sends the message without a taskType header
func (r *repository) InsertOutbox(ctx context.Context, session Session, id, conversationId uuid.UUID, topic, taskType string, payload []byte) error {
	_, err := session.ExecContext(ctx,
		`INSERT INTO outbox (id, conversation_id, topic, task_type, payload) VALUES (?, ?, ?, NULLIF(?, ''), ?)`,
		id[:], conversationId[:], topic, taskType, string(payload))
	if err != nil {
		slog.Error("fail to insert outbox", "err", err, "topic", topic, "taskType", taskType, "conversationId", conversationId)
		return err
	}
	return nil
}

// DeleteOutbox deletes by the shard key too, so vtgate routes it to a single shard
func (r *repository) DeleteOutbox(ctx context.Context, session Session, id, conversationId uuid.UUID) error {
	_, err := session.ExecContext(ctx,
		`DELETE FROM outbox WHERE id = ? AND conversation_id = ?`,
		id[:], conversationId[:])
	if err != nil {
		slog.Error("fail to delete outbox", "err", err, "conversationId", conversationId)
		return err
	}
	return nil
}
