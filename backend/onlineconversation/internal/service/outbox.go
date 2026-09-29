package service

import (
	"backend/onlineconversation/internal/repository"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	scheduledNotificationTopic    = "scheduled-notification"
	onlineConversationTopic       = "online-conversation"
	reportTaskType                = "report"
	notificationTimeMinusInterval = 15 * time.Minute
)

var ErrTooLateForNotification = errors.New("conversation starts within 15 minutes")

// publish writes the value to the outbox, the CDC job sends it to topic with taskType as the "taskType" header,
// an empty taskType sends no header
func (s *service) publish(ctx context.Context, tx repository.Session, conversationId uuid.UUID, topic, taskType string, value []byte) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	err = s.repository.InsertOutbox(ctx, tx, id, conversationId, topic, taskType, value)
	if err != nil {
		return err
	}
	return s.repository.DeleteOutbox(ctx, tx, id, conversationId)
}
