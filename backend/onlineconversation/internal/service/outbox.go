package service

import (
	"backend/common/payload"
	"backend/onlineconversation/internal/projection"
	"backend/onlineconversation/internal/repository"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	scheduledNotificationTopic    = "scheduled-notification"
	notificationTimeMinusInterval = 15 * time.Minute
)

var ErrTooLateForNotification = errors.New("conversation starts within 15 minutes")

func (s *service) publish(ctx context.Context, tx repository.Session, conversationId uuid.UUID, topic string, value []byte) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	err = s.repository.InsertOutbox(ctx, tx, id, conversationId, topic, value)
	if err != nil {
		return err
	}
	return s.repository.DeleteOutbox(ctx, tx, id, conversationId)
}

func (s *service) publishScheduleNotification(ctx context.Context, tx repository.Session, conversationId, memberId uuid.UUID, detail projection.Detail) error {
	return s.publish(ctx, tx, conversationId, scheduledNotificationTopic, payload.Marshal(payload.NotificationScheduling{
		PartitionId:   conversationId,
		KeyId:         memberId,
		ScheduledTime: detail.Time.Add(-notificationTimeMinusInterval).UnixMilli(),
		Contents:      map[int]string{0: about(detail)},
		Type:          "online-conversation",
	}))
}

func (s *service) publishCancelNotification(ctx context.Context, tx repository.Session, conversationId, memberId uuid.UUID) error {
	return s.publish(ctx, tx, conversationId, scheduledNotificationTopic, payload.Marshal(payload.NotificationScheduling{
		PartitionId: conversationId,
		KeyId:       memberId,
		Type:        "cancel",
	}))
}

func about(d projection.Detail) string {
	aboutRaw := []rune(d.Novel + d.Play + d.Poem + d.ShortStory + d.Film + d.WrittenBy)
	if len(aboutRaw) > 6 {
		return string(aboutRaw[:6]) + "..."
	}
	return string(aboutRaw)
}
