package test

import (
	"backend/common"
	"backend/common/mocks"
	"backend/onlineconversation/internal/dto"
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

func newReportService(t *testing.T, repo *MockRepository, m *MockModerationClient) service.Service {
	return service.NewService(repo, mocks.NewMockSyncProducer(t), mocks.NewMockAsyncProducer(t), m)
}

func TestReportConversation_keysByConversationWithoutValue(t *testing.T) {
	p := mocks.NewMockSyncProducer(t)
	p.EXPECT().Commit("online-conversation", conversationId[:], []byte(nil), mock.Anything).Return(nil)

	s := service.NewService(NewMockRepository(t), p, mocks.NewMockAsyncProducer(t), NewMockModerationClient(t))

	require.NoError(t, s.ReportConversation(context.Background(), conversationId, memberId))
}

func TestManageReport_evaluatedConversationIsSkipped(t *testing.T) {
	repo, m := NewMockRepository(t), NewMockModerationClient(t)
	repo.EXPECT().Tx().Return(nil)
	repo.EXPECT().FindReportTarget(mock.Anything, mock.Anything, conversationId).
		Return(projection.ReportTarget{IsEvaluated: true}, nil)

	require.NoError(t, newReportService(t, repo, m).ManageReport(context.Background(), conversationId))
}

func TestManageReport_deletedConversationIsDropped(t *testing.T) {
	repo, m := NewMockRepository(t), NewMockModerationClient(t)
	repo.EXPECT().Tx().Return(nil)
	repo.EXPECT().FindReportTarget(mock.Anything, mock.Anything, conversationId).
		Return(projection.ReportTarget{}, sql.ErrNoRows)

	require.NoError(t, newReportService(t, repo, m).ManageReport(context.Background(), conversationId))
}

func TestManageReport_cleanVerdictIsStoredAndConversationKept(t *testing.T) {
	repo, tx, m := NewMockRepository(t), NewMockTx(t), NewMockModerationClient(t)
	updatedAt := time.Now().UTC()
	target := projection.ReportTarget{Contents: projection.Contents{Novel: "Macbeth"}, UpdatedAt: &updatedAt}
	verdict := dto.Verdict{Reason: "literary discussion", Category: "none", Violation: false}
	repo.EXPECT().Tx().Return(nil)
	repo.EXPECT().FindReportTarget(mock.Anything, mock.Anything, conversationId).Return(target, nil)
	m.EXPECT().Evaluate(mock.Anything, target.Contents).Return(verdict, nil)
	m.EXPECT().Model().Return("qwen3:8b")
	repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
	repo.EXPECT().MarkEvaluatedIfUnchanged(mock.Anything, tx, conversationId, &updatedAt).Return(true, nil)
	repo.EXPECT().InsertVerdict(mock.Anything, tx, conversationId, target, "qwen3:8b", verdict).Return(nil)
	tx.EXPECT().Commit().Return(nil)
	tx.EXPECT().Rollback().Return(nil) // deferred, a no-op after commit

	require.NoError(t, newReportService(t, repo, m).ManageReport(context.Background(), conversationId))
}

func TestManageReport_violationIsStoredAndDeletesConversationWithMembers(t *testing.T) {
	repo, tx, m := NewMockRepository(t), NewMockTx(t), NewMockModerationClient(t)
	verdict := dto.Verdict{Reason: "sells a paid course", Category: "advertising", Violation: true}
	repo.EXPECT().Tx().Return(nil)
	repo.EXPECT().FindReportTarget(mock.Anything, mock.Anything, conversationId).Return(projection.ReportTarget{}, nil)
	m.EXPECT().Evaluate(mock.Anything, mock.Anything).Return(verdict, nil)
	m.EXPECT().Model().Return("qwen3:8b")
	repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
	repo.EXPECT().MarkEvaluatedIfUnchanged(mock.Anything, tx, conversationId, mock.Anything).Return(true, nil)
	repo.EXPECT().InsertVerdict(mock.Anything, tx, conversationId, mock.Anything, "qwen3:8b", verdict).Return(nil)
	repo.EXPECT().DeleteConversation(mock.Anything, tx, conversationId).Return(nil)
	repo.EXPECT().DeleteConversationMembers(mock.Anything, tx, conversationId).Return(nil)
	events := expectOutbox(t, repo, tx, 1)
	tx.EXPECT().Commit().Return(nil)
	tx.EXPECT().Rollback().Return(nil)

	require.NoError(t, newReportService(t, repo, m).ManageReport(context.Background(), conversationId))

	require.Len(t, *events, 1)
	assert.Equal(t, common.NotificationScheduling{PartitionId: conversationId, Type: "cancel-all"}, (*events)[0])
}

func TestManageReport_failedEvaluationIsRetriedWithoutTransaction(t *testing.T) {
	repo, m := NewMockRepository(t), NewMockModerationClient(t)
	repo.EXPECT().Tx().Return(nil)
	repo.EXPECT().FindReportTarget(mock.Anything, mock.Anything, conversationId).Return(projection.ReportTarget{}, nil)
	m.EXPECT().Evaluate(mock.Anything, mock.Anything).Return(dto.Verdict{}, errors.New("ollama timeout"))

	err := newReportService(t, repo, m).ManageReport(context.Background(), conversationId)

	assert.EqualError(t, err, "ollama timeout")
}

func TestManageReport_contentChangedWithCleanVerdictIsRetried(t *testing.T) {
	repo, tx, m := NewMockRepository(t), NewMockTx(t), NewMockModerationClient(t)
	repo.EXPECT().Tx().Return(nil)
	repo.EXPECT().FindReportTarget(mock.Anything, mock.Anything, conversationId).Return(projection.ReportTarget{}, nil)
	m.EXPECT().Evaluate(mock.Anything, mock.Anything).Return(dto.Verdict{Category: "none", Violation: false}, nil)
	repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
	repo.EXPECT().MarkEvaluatedIfUnchanged(mock.Anything, tx, conversationId, mock.Anything).Return(false, nil)
	tx.EXPECT().Rollback().Return(nil)

	err := newReportService(t, repo, m).ManageReport(context.Background(), conversationId)

	assert.EqualError(t, err, "conversation contents was okay, but contents changed during report evaluation",
		"nothing is stored, the retryer evaluates the new content")
}

func TestManageReport_contentChangedWithViolationIsStillApplied(t *testing.T) {
	repo, tx, m := NewMockRepository(t), NewMockTx(t), NewMockModerationClient(t)
	verdict := dto.Verdict{Reason: "slur in the description", Category: "hate", Violation: true}
	repo.EXPECT().Tx().Return(nil)
	repo.EXPECT().FindReportTarget(mock.Anything, mock.Anything, conversationId).Return(projection.ReportTarget{}, nil)
	m.EXPECT().Evaluate(mock.Anything, mock.Anything).Return(verdict, nil)
	m.EXPECT().Model().Return("qwen3:8b")
	repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
	repo.EXPECT().MarkEvaluatedIfUnchanged(mock.Anything, tx, conversationId, mock.Anything).Return(false, nil)
	repo.EXPECT().InsertVerdict(mock.Anything, tx, conversationId, mock.Anything, "qwen3:8b", verdict).Return(nil)
	expectOutbox(t, repo, tx, 1)
	repo.EXPECT().DeleteConversation(mock.Anything, tx, conversationId).Return(nil)
	repo.EXPECT().DeleteConversationMembers(mock.Anything, tx, conversationId).Return(nil)
	tx.EXPECT().Commit().Return(nil)
	tx.EXPECT().Rollback().Return(nil)

	require.NoError(t, newReportService(t, repo, m).ManageReport(context.Background(), conversationId),
		"the violating content was really posted, so it is deleted without a retry")
}

func TestManageReport_failedMarkIsReturnedForRetry(t *testing.T) {
	repo, tx, m := NewMockRepository(t), NewMockTx(t), NewMockModerationClient(t)
	repo.EXPECT().Tx().Return(nil)
	repo.EXPECT().FindReportTarget(mock.Anything, mock.Anything, conversationId).Return(projection.ReportTarget{}, nil)
	m.EXPECT().Evaluate(mock.Anything, mock.Anything).Return(dto.Verdict{}, nil)
	repo.EXPECT().BeginTx(mock.Anything).Return(tx, nil)
	repo.EXPECT().MarkEvaluatedIfUnchanged(mock.Anything, tx, conversationId, mock.Anything).Return(false, errors.New("db down"))
	tx.EXPECT().Rollback().Return(nil)

	err := newReportService(t, repo, m).ManageReport(context.Background(), conversationId)

	assert.EqualError(t, err, "db down")
}
