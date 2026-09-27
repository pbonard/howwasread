package test

import (
	"backend/common/mocks"
	"backend/common/payload"
	"backend/fcmnotification/internal/service"
	"context"
	"errors"
	"testing"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

var (
	messageId = uuid.New()
	memberId  = uuid.New()
	idKey     = string(messageId[:])
	notiKey   = string(uint8(1))
)

func newService(t *testing.T) (service.Service, *MockRepository, *MockFCMClient) {
	repo, fcm := NewMockRepository(t), NewMockFCMClient(t)
	return service.NewService(repo, mocks.NewMockProducer(t), fcm), repo, fcm
}

func value(p payload.NotificationMessage) []byte {
	return payload.Marshal(p)
}

func TestSendNotification_sendsOnceAndMarksAsSent(t *testing.T) {
	s, repo, fcm := newService(t)
	repo.EXPECT().DidNotification(mock.Anything, idKey, notiKey).Return(false, nil)
	fcm.EXPECT().Send(mock.Anything, []string{"token"}, "Seoul", "alice: hello", "https://img").Return(nil, nil)
	repo.EXPECT().MarkNotification(mock.Anything, idKey, notiKey).Return(nil)

	s.SendNotification(context.Background(), messageId, 1, value(payload.NotificationMessage{
		TokenMap: map[string]uuid.UUID{"token": memberId},
		Title:    "Seoul", SubTitle: "alice", Text: "hello", ImageURL: "https://img",
	}))
}

func TestSendNotification_rejectedTokensAreRemoved(t *testing.T) {
	s, repo, fcm := newService(t)
	other := uuid.New()
	repo.EXPECT().DidNotification(mock.Anything, idKey, notiKey).Return(false, nil)
	fcm.EXPECT().Send(mock.Anything, mock.Anything, "x", "y", "").Return([]string{"stale"}, nil)
	repo.EXPECT().MarkNotification(mock.Anything, idKey, notiKey).Return(nil)
	repo.EXPECT().RemoveNotificationInfoByIdAndToken(mock.Anything, gocql.UUID(other), "stale").Return(nil)

	s.SendNotification(context.Background(), messageId, 1, value(payload.NotificationMessage{
		TokenMap: map[string]uuid.UUID{"token": memberId, "stale": other}, Title: "x", Text: "y",
	}))
}

func TestSendNotification_alreadySentIsSkipped(t *testing.T) {
	s, repo, _ := newService(t)
	repo.EXPECT().DidNotification(mock.Anything, idKey, notiKey).Return(true, nil)

	s.SendNotification(context.Background(), messageId, 1, value(payload.NotificationMessage{Title: "x"}))
}

func TestSendNotification_idempotencyCheckErrorSkipsSending(t *testing.T) {
	s, repo, _ := newService(t)
	repo.EXPECT().DidNotification(mock.Anything, idKey, notiKey).Return(false, errors.New("valkey down"))

	s.SendNotification(context.Background(), messageId, 1, value(payload.NotificationMessage{Title: "x"}))
}

func TestSendNotification_malformedPayloadIsSkipped(t *testing.T) {
	s, repo, _ := newService(t)
	repo.EXPECT().DidNotification(mock.Anything, idKey, notiKey).Return(false, nil)

	s.SendNotification(context.Background(), messageId, 1, []byte("{not json"))
}

func TestSendNotification_failedSendIsNotMarked(t *testing.T) {
	s, repo, fcm := newService(t)
	repo.EXPECT().DidNotification(mock.Anything, idKey, notiKey).Return(false, nil)
	fcm.EXPECT().Send(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("fcm down"))

	s.SendNotification(context.Background(), messageId, 1, value(payload.NotificationMessage{
		TokenMap: map[string]uuid.UUID{"token": memberId}, Title: "x", Text: "y",
	}))
}
