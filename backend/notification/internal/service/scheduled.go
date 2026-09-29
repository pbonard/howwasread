package service

import (
	"backend/common/payload"
	"context"
	"fmt"

	"github.com/google/uuid"
)

const scheduledNotificationId uint8 = 2

func (s *service) PreprocessScheduledNotification(ctx context.Context, partitionId uuid.UUID, notifications map[uuid.UUID]map[int]string, contents map[int]string) error {
	memberIds := make([]uuid.UUID, 0, len(notifications))
	for memberId, _ := range notifications {
		memberIds = append(memberIds, memberId)
	}
	apntm, fcmtm, err := s.getEachTokenMap(ctx, memberIds)
	if err != nil {
		return err
	}
	p := payload.NotificationMessage{
		TokenMap: fcmtm,
		Title:    "Conversation starts soon",
		Text: fmt.Sprintf("You can now enter the conversation about %s and talk!",
			contents[0]),
	}
	kafkaKey := append(partitionId[:], scheduledNotificationId)
	if len(fcmtm) > 0 {
		p.TokenMap = fcmtm
		err = s.producer.Commit("fcm-notification", kafkaKey, payload.Marshal(p), nil)
		if err != nil {
			return err
		}
	}
	if len(apntm) > 0 {
		p.TokenMap = apntm
		err = s.producer.Commit("apn-notification", kafkaKey, payload.Marshal(p), nil)
		if err != nil {
			return err
		}
	}
	return nil
}
