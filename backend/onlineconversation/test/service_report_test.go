package test

import (
	"backend/common/mocks"
	"backend/onlineconversation/internal/projection"
	"backend/onlineconversation/internal/service"
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestReportConversation_keysByConversationWithoutValue(t *testing.T) {
	p := mocks.NewMockSyncProducer(t)
	p.EXPECT().Commit("online-conversation", conversationId[:], []byte(nil), mock.Anything).Return(nil)

	s := service.NewService(NewMockRepository(t), p, mocks.NewMockAsyncProducer(t))

	require.NoError(t, s.ReportConversation(context.Background(), conversationId, memberId))
}

func TestManageReport_evaluatedConversationIsSkipped(t *testing.T) {
	repo := NewMockRepository(t)
	repo.EXPECT().Tx().Return(nil)
	repo.EXPECT().FindReportTarget(mock.Anything, mock.Anything, conversationId).
		Return(projection.ReportTarget{IsEvaluated: true}, nil)

	require.NoError(t, newService(t, repo).ManageReport(context.Background(), conversationId))
}

func TestManageReport_marksWithTheUpdatedAtReadBeforeEvaluation(t *testing.T) {
	repo, tx := NewMockRepository(t), NewMockTx(t)
	updatedAt := time.Now().UTC()
	repo.EXPECT().Tx().Return(nil)
	repo.EXPECT().FindReportTarget(mock.Anything, mock.Anything, conversationId).
		Return(projection.ReportTarget{UpdatedAt: &updatedAt}, nil)
	repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
	repo.EXPECT().MarkEvaluatedIfUnchanged(mock.Anything, tx, conversationId, &updatedAt).Return(true, nil)
	tx.EXPECT().Commit().Return(nil)
	tx.EXPECT().Rollback().Return(nil) // deferred, a no-op after commit

	require.NoError(t, newService(t, repo).ManageReport(context.Background(), conversationId))
}

func TestManageReport_contentChangedDuringEvaluationIsRetried(t *testing.T) {
	repo, tx := NewMockRepository(t), NewMockTx(t)
	repo.EXPECT().Tx().Return(nil)
	repo.EXPECT().FindReportTarget(mock.Anything, mock.Anything, conversationId).
		Return(projection.ReportTarget{}, nil)
	repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
	repo.EXPECT().MarkEvaluatedIfUnchanged(mock.Anything, tx, conversationId, mock.Anything).Return(false, nil)
	tx.EXPECT().Rollback().Return(nil)

	err := newService(t, repo).ManageReport(context.Background(), conversationId)

	assert.EqualError(t, err, "conversation changed during report evaluation",
		"the error sends the report to the retryer, which evaluates the new content")
}

func TestManageReport_deletedConversationIsDropped(t *testing.T) {
	repo := NewMockRepository(t)
	repo.EXPECT().Tx().Return(nil)
	repo.EXPECT().FindReportTarget(mock.Anything, mock.Anything, conversationId).
		Return(projection.ReportTarget{}, sql.ErrNoRows)

	require.NoError(t, newService(t, repo).ManageReport(context.Background(), conversationId))
}

func TestManageReport_failedMarkIsReturnedForRetry(t *testing.T) {
	repo, tx := NewMockRepository(t), NewMockTx(t)
	repo.EXPECT().Tx().Return(nil)
	repo.EXPECT().FindReportTarget(mock.Anything, mock.Anything, conversationId).
		Return(projection.ReportTarget{}, nil)
	repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
	repo.EXPECT().MarkEvaluatedIfUnchanged(mock.Anything, tx, conversationId, mock.Anything).Return(false, errors.New("db down"))
	tx.EXPECT().Rollback().Return(nil)

	err := newService(t, repo).ManageReport(context.Background(), conversationId)

	assert.EqualError(t, err, "db down")
}
