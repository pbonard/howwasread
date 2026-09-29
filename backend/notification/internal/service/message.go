package service

import (
	"backend/common/payload"
	"bytes"
	"context"
	"fmt"
	"log"
	"log/slog"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/google/uuid"
)

func (s *service) PreprocessMessageNotification(
	ctx context.Context,
	notificationId uint8,
	messageId uuid.UUID,
	toIds [][]byte,
	roomId, fromId uuid.UUID,
	contentType string,
	content []string,
) error {
	log.Print("start notify message...")
	if len(toIds) == 0 {
		return nil
	}
	tIds := make([]uuid.UUID, 0, len(toIds))
	for _, toId := range toIds {
		tIds = append(tIds, uuid.UUID(toId))
	}
	apntm, fcmtm, err := s.getEachTokenMap(ctx, tIds)
	if err != nil {
		return err
	}
	senderName, err := s.repository.FindNameById(ctx, gocql.UUID(fromId))
	if err != nil {
		return err
	}
	var roomName string
	if !bytes.Equal(toIds[0], roomId[:]) {
		roomName, err = s.repository.FindRoomNameById(ctx, gocql.UUID(roomId))
	}
	if err != nil {
		return err
	}

	p := payload.NotificationMessage{
		Title: senderName,
		Text:  content[0],
	}
	if roomName != "" {
		p.Title = roomName
		p.SubTitle = senderName
	}

	if contentType == "image" {
		imageURL, err1 := s.cdnClient.SignedURL(contentType, content[0])
		if err1 != nil {
			slog.Error("fail to generate Signed URL", "err", err1)
			return err1
		}
		p.ImageURL = imageURL
	}

	if contentType != "text" {
		p.Text = fmt.Sprintf("(%v)", contentType)
	}

	kafkaKey := append(messageId[:], notificationId)
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
