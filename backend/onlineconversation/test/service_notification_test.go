package test

import (
	"backend/common"
	"backend/onlineconversation/internal/projection"
	"backend/onlineconversation/internal/repository"
	"backend/onlineconversation/internal/service"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// expectOutbox expects one outbox insert and its delete on tx, and returns the published events
func expectOutbox(t *testing.T, repo *MockRepository, tx *MockTx, times int) *[]common.NotificationScheduling {
	var events []common.NotificationScheduling
	var ids []uuid.UUID
	repo.EXPECT().InsertOutbox(mock.Anything, tx, mock.Anything, conversationId, "scheduled-notification", "", mock.Anything).
		Run(func(_ context.Context, _ repository.Session, id, _ uuid.UUID, _, _ string, value []byte) {
			var e common.NotificationScheduling
			require.NoError(t, json.Unmarshal(value, &e))
			events = append(events, e)
			ids = append(ids, id)
		}).Return(nil).Times(times)
	repo.EXPECT().DeleteOutbox(mock.Anything, tx, mock.Anything, conversationId).
		Run(func(_ context.Context, _ repository.Session, id, _ uuid.UUID) {
			assert.Contains(t, ids, id, "deletes the row it inserted")
		}).Return(nil).Times(times)
	return &events
}

func TestScheduleNotification_publishesAFullScheduleEvent(t *testing.T) {
	repo, tx := NewMockRepository(t), NewMockTx(t)
	start := time.Now().UTC().Add(time.Hour)
	repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
	repo.EXPECT().FindConversationDetail(mock.Anything, tx, conversationId, memberId).
		Return(projection.Detail{Novel: "Hamlet", WrittenBy: "shakespeare", Time: start}, nil)
	repo.EXPECT().AddNotificationId(mock.Anything, tx, conversationId, memberId).Return(nil)
	events := expectOutbox(t, repo, tx, 1)
	tx.EXPECT().Commit().Return(nil)
	tx.EXPECT().Rollback().Return(nil)

	require.NoError(t, newService(t, repo).ScheduleNotification(context.Background(), memberId, conversationId))

	require.Len(t, *events, 1)
	e := (*events)[0]
	assert.Equal(t, "online-conversation", e.Type)
	assert.Equal(t, conversationId, e.PartitionId)
	assert.Equal(t, memberId, e.KeyId)
	assert.Equal(t, start.Add(-15*time.Minute).UnixMilli(), e.ScheduledTime)
	assert.Equal(t, map[int]string{0: "Hamlet..."}, e.Contents)
}

func TestScheduleNotification_rejectedWithin15MinutesOfStart(t *testing.T) {
	repo, tx := NewMockRepository(t), NewMockTx(t)
	repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
	repo.EXPECT().FindConversationDetail(mock.Anything, tx, conversationId, memberId).
		Return(projection.Detail{Time: time.Now().UTC().Add(14 * time.Minute)}, nil)
	tx.EXPECT().Rollback().Return(nil)

	err := newService(t, repo).ScheduleNotification(context.Background(), memberId, conversationId)

	assert.ErrorIs(t, err, service.ErrTooLateForNotification)
}

func TestCancelNotification_publishesACancelEvent(t *testing.T) {
	repo, tx := NewMockRepository(t), NewMockTx(t)
	repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
	repo.EXPECT().RemoveNotificationId(mock.Anything, tx, conversationId, memberId).Return(nil)
	events := expectOutbox(t, repo, tx, 1)
	tx.EXPECT().Commit().Return(nil)
	tx.EXPECT().Rollback().Return(nil)

	require.NoError(t, newService(t, repo).CancelNotification(context.Background(), memberId, conversationId))

	require.Len(t, *events, 1)
	assert.Equal(t, common.NotificationScheduling{PartitionId: conversationId, KeyId: memberId, Type: "cancel"}, (*events)[0])
}

func TestDeleteConversation_cancelsEverySubscribersReminder(t *testing.T) {
	repo, tx := NewMockRepository(t), NewMockTx(t)
	alice, bob := uuid.New(), uuid.New()
	repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
	repo.EXPECT().FindNotificationIds(mock.Anything, tx, conversationId).Return([]uuid.UUID{alice, bob}, nil)
	repo.EXPECT().DeleteOnlineConversationIfModerator(mock.Anything, tx, conversationId, memberId).Return(true, nil)
	events := expectOutbox(t, repo, tx, 2)
	tx.EXPECT().Commit().Return(nil)
	tx.EXPECT().Rollback().Return(nil)

	require.NoError(t, newService(t, repo).DeleteConversation(context.Background(), memberId, conversationId))

	require.Len(t, *events, 2)
	for i, member := range []uuid.UUID{alice, bob} {
		assert.Equal(t, "cancel", (*events)[i].Type)
		assert.Equal(t, member, (*events)[i].KeyId)
	}
}

func TestDeleteConversation_nonModeratorPublishesNothing(t *testing.T) {
	repo, tx := NewMockRepository(t), NewMockTx(t)
	repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
	repo.EXPECT().FindNotificationIds(mock.Anything, tx, conversationId).Return([]uuid.UUID{uuid.New()}, nil)
	repo.EXPECT().DeleteOnlineConversationIfModerator(mock.Anything, tx, conversationId, memberId).Return(false, nil)
	tx.EXPECT().Rollback().Return(nil)

	err := newService(t, repo).DeleteConversation(context.Background(), memberId, conversationId)

	assert.EqualError(t, err, "can't delete conversation")
}

func TestReportConversation_publishesWithReportTaskType(t *testing.T) {
	repo, tx := NewMockRepository(t), NewMockTx(t)
	repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
	repo.EXPECT().InsertOutbox(mock.Anything, tx, mock.Anything, conversationId, "online-conversation", "report", mock.Anything).Return(nil)
	repo.EXPECT().DeleteOutbox(mock.Anything, tx, mock.Anything, conversationId).Return(nil)
	tx.EXPECT().Commit().Return(nil)

	require.NoError(t, newService(t, repo).ReportConversation(context.Background(), conversationId))
}
