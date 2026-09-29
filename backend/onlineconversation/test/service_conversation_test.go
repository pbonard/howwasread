package test

import (
	"backend/common/mocks"
	"backend/onlineconversation/internal/dto"
	"backend/onlineconversation/internal/projection"
	"backend/onlineconversation/internal/repository"
	"backend/onlineconversation/internal/service"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	memberId       = uuid.New()
	conversationId = uuid.New()
	errDB          = errors.New("mysql down")
)

func newService(t *testing.T, repo *MockRepository) service.Service {
	return service.NewService(repo, mocks.NewMockSyncProducer(t), mocks.NewMockAsyncProducer(t))
}

// ---------------------------------------------------------------- create

func TestCreateConversation_insertsAllInOneTransaction(t *testing.T) {
	repo, tx := NewMockRepository(t), NewMockTx(t)
	req := dto.CreateConversationRequest{WrittenBy: "shakespeare", Capacity: 6}
	var insertedId uuid.UUID

	repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
	repo.EXPECT().InsertConversation(mock.Anything, tx, mock.Anything, req).
		Run(func(_ context.Context, _ repository.Session, id uuid.UUID, _ dto.CreateConversationRequest) {
			insertedId = id
		}).
		Return(nil)
	repo.EXPECT().InsertModerator(mock.Anything, tx, mock.Anything, memberId).Return(nil)
	repo.EXPECT().InsertRegistrant(mock.Anything, tx, mock.Anything, memberId).Return(nil)
	tx.EXPECT().Commit().Return(nil)
	tx.EXPECT().Rollback().Return(nil) // deferred, a no-op after commit

	resp, err := newService(t, repo).CreateConversation(context.Background(), memberId, req)

	require.NoError(t, err)
	assert.Equal(t, insertedId, resp["conversationId"])
	assert.Equal(t, uuid.Version(7), insertedId.Version())
}

func TestCreateConversation_failedInsertRollsBack(t *testing.T) {
	repo, tx := NewMockRepository(t), NewMockTx(t)
	repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
	repo.EXPECT().InsertConversation(mock.Anything, tx, mock.Anything, mock.Anything).Return(nil)
	repo.EXPECT().InsertModerator(mock.Anything, tx, mock.Anything, memberId).Return(errDB)
	tx.EXPECT().Rollback().Return(nil)

	resp, err := newService(t, repo).CreateConversation(context.Background(), memberId, dto.CreateConversationRequest{})

	assert.ErrorIs(t, err, errDB)
	assert.Nil(t, resp)
}

// ---------------------------------------------------------------- moderator guarded updates

func TestModeratorGuardedActions(t *testing.T) {
	banId := uuid.New()
	actions := []struct {
		name   string
		expect func(r *MockRepository_Expecter) *mock.Call
		call   func(s service.Service) error
		denied string
	}{
		{
			name: "update",
			expect: func(r *MockRepository_Expecter) *mock.Call {
				return r.UpdateConversationIfModerator(mock.Anything, mock.Anything, memberId, mock.Anything).Call
			},
			call: func(s service.Service) error {
				return s.UpdateConversation(context.Background(), memberId, dto.UpdateConversationRequest{Id: conversationId})
			},
			denied: "can't update conversation",
		},
		{
			name: "ban",
			expect: func(r *MockRepository_Expecter) *mock.Call {
				return r.AddBanIdIfModerator(mock.Anything, mock.Anything, conversationId, memberId, banId).Call
			},
			call: func(s service.Service) error {
				return s.BanParticipant(context.Background(), memberId, conversationId, banId)
			},
			denied: "can't ban participant",
		},
	}

	for _, a := range actions {
		t.Run(a.name+" by moderator", func(t *testing.T) {
			repo := NewMockRepository(t)
			repo.EXPECT().Tx().Return(nil)
			a.expect(repo.EXPECT()).Return(true, nil)

			assert.NoError(t, a.call(newService(t, repo)))
		})
		t.Run(a.name+" by non moderator", func(t *testing.T) {
			repo := NewMockRepository(t)
			repo.EXPECT().Tx().Return(nil)
			a.expect(repo.EXPECT()).Return(false, nil)

			assert.EqualError(t, a.call(newService(t, repo)), a.denied)
		})
		t.Run(a.name+" repository error", func(t *testing.T) {
			repo := NewMockRepository(t)
			repo.EXPECT().Tx().Return(nil)
			a.expect(repo.EXPECT()).Return(false, errDB)

			assert.ErrorIs(t, a.call(newService(t, repo)), errDB)
		})
	}
}

// ---------------------------------------------------------------- detail

func TestGetConversationDetail_canEnter(t *testing.T) {
	// the start time is set relative to now, a minute away from each boundary
	tests := []struct {
		name       string
		startIn    time.Duration
		registrant bool
		banned     bool
		want       bool
	}{
		{"more than 15 minutes before start", 16 * time.Minute, true, false, false},
		{"registrant within 15 minutes before start", 14 * time.Minute, true, false, true},
		{"non registrant before 10 minutes after start", -9 * time.Minute, false, false, false},
		{"non registrant 10 minutes after start", -11 * time.Minute, false, false, true},
		{"banned registrant", 0, true, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now().UTC().Add(tt.startIn)
			repo := NewMockRepository(t)
			repo.EXPECT().Tx().Return(nil)
			repo.EXPECT().FindConversationDetail(mock.Anything, mock.Anything, conversationId, memberId).Return(
				projection.Detail{Novel: "Hamlet", Capacity: 6, Time: start, IsRegistrant: tt.registrant, IsBanned: tt.banned}, nil)

			resp, err := newService(t, repo).GetConversationDetail(context.Background(), conversationId, memberId)

			require.NoError(t, err)
			assert.Equal(t, tt.want, resp.CanEnter)
			assert.Equal(t, "Hamlet", resp.Novel)
			assert.Equal(t, 6, resp.Capacity)
			assert.Equal(t, tt.registrant, resp.IsRegistrant)
		})
	}
}

// ---------------------------------------------------------------- register / deregister

func TestRegisterOnlineConversation(t *testing.T) {
	t.Run("seat available", func(t *testing.T) {
		repo, tx := NewMockRepository(t), NewMockTx(t)
		repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
		repo.EXPECT().TryIncrementRegistrants(mock.Anything, tx, conversationId).Return(true, nil)
		repo.EXPECT().InsertRegistrant(mock.Anything, tx, conversationId, memberId).Return(nil)
		tx.EXPECT().Commit().Return(nil)
		tx.EXPECT().Rollback().Return(nil)

		assert.NoError(t, newService(t, repo).RegisterConversation(context.Background(), memberId, conversationId))
	})
	t.Run("full", func(t *testing.T) {
		repo, tx := NewMockRepository(t), NewMockTx(t)
		repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
		repo.EXPECT().TryIncrementRegistrants(mock.Anything, tx, conversationId).Return(false, nil)
		tx.EXPECT().Rollback().Return(nil)

		assert.EqualError(t, newService(t, repo).RegisterConversation(context.Background(), memberId, conversationId),
			"already fully registered")
	})
}

func TestDeregisterOnlineConversation(t *testing.T) {
	t.Run("removes and decrements", func(t *testing.T) {
		repo, tx := NewMockRepository(t), NewMockTx(t)
		repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
		repo.EXPECT().RemoveRegistrantId(mock.Anything, tx, conversationId, memberId).Return(nil)
		repo.EXPECT().DecrementRegistrants(mock.Anything, tx, conversationId).Return(nil)
		tx.EXPECT().Commit().Return(nil)
		tx.EXPECT().Rollback().Return(nil)

		assert.NoError(t, newService(t, repo).DeregisterConversation(context.Background(), memberId, conversationId))
	})
	t.Run("decrement fails", func(t *testing.T) {
		repo, tx := NewMockRepository(t), NewMockTx(t)
		repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
		repo.EXPECT().RemoveRegistrantId(mock.Anything, tx, conversationId, memberId).Return(nil)
		repo.EXPECT().DecrementRegistrants(mock.Anything, tx, conversationId).Return(errDB)
		tx.EXPECT().Rollback().Return(nil)

		assert.ErrorIs(t, newService(t, repo).DeregisterConversation(context.Background(), memberId, conversationId), errDB)
	})
}

// ---------------------------------------------------------------- notification

func TestGetConversationDetail_updatedAtIsOmittedUntilUpdated(t *testing.T) {
	updated := time.Date(2026, 9, 24, 1, 0, 0, 0, time.UTC)
	tests := map[string]struct {
		stored   *time.Time
		want     time.Time
		wantJSON bool
	}{
		"never updated": {stored: nil, want: time.Time{}, wantJSON: false},
		"updated":       {stored: &updated, want: updated, wantJSON: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			repo := NewMockRepository(t)
			repo.EXPECT().Tx().Return(nil)
			repo.EXPECT().FindConversationDetail(mock.Anything, mock.Anything, conversationId, memberId).Return(
				projection.Detail{Time: time.Now().UTC(), UpdatedAt: tt.stored}, nil)

			resp, err := newService(t, repo).GetConversationDetail(context.Background(), conversationId, memberId)

			require.NoError(t, err)
			assert.Equal(t, tt.want, resp.UpdatedAt)
			body, err := json.Marshal(resp)
			require.NoError(t, err)
			assert.Equal(t, tt.wantJSON, strings.Contains(string(body), `"updatedAt"`))
		})
	}
}
