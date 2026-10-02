package service

import (
	"backend/common"
	"backend/onlineconversation/internal/dto"
	"backend/onlineconversation/internal/repository"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

func (s *service) CreateConversation(ctx context.Context, memberId uuid.UUID, req dto.CreateConversationRequest) (map[string]uuid.UUID, error) {
	conversationId, err := uuid.NewV7()
	if err != nil {
		slog.Error("fail to create uuid v7 for online conversation id", "err", err)
		return nil, err
	}

	tx, err := s.repository.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	err = s.repository.InsertConversation(ctx, tx,
		conversationId,
		req)
	if err != nil {
		return nil, err
	}
	err = s.repository.InsertModerator(ctx, tx, conversationId, memberId)
	if err != nil {
		return nil, err
	}
	err = s.repository.InsertRegistrant(ctx, tx, conversationId, memberId)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		slog.Error("fail to commit transaction for create conversation", "err", err)
		return nil, err
	}
	slog.Info("success to create conversation")
	return map[string]uuid.UUID{"conversationId": conversationId}, nil
}

func (s *service) UpdateConversation(ctx context.Context, memberId uuid.UUID, req dto.UpdateConversationRequest) error {
	ok, err := s.repository.UpdateConversationIfModerator(ctx, s.repository.Tx(), memberId, req)
	if err != nil {
		return err
	}
	if !ok {
		err = errors.New("can't update conversation")
		slog.Warn("update online conversation failed, ui error or api abuse attempt",
			"conversationId", req.Id,
			"memberId", memberId)
		return err
	}
	return nil
}

func (s *service) DeleteConversation(ctx context.Context, memberId, conversationId uuid.UUID) error {
	tx, err := s.repository.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ok, err := s.repository.DeleteConversationIfModerator(ctx, tx, conversationId, memberId)
	if err != nil {
		return err
	}
	if !ok {
		err = errors.New("can't delete conversation")
		slog.Warn("delete online conversation failed, ui error or api abuse attempt",
			"conversationId", conversationId,
			"memberId", memberId)
		return err
	}
	err = s.removeConversationRest(ctx, tx, conversationId)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		slog.Error("fail to commit transaction for delete conversation", "err", err)
		return err
	}
	return nil
}

func (s *service) removeConversationRest(ctx context.Context, tx repository.Tx, conversationId uuid.UUID) error {
	err := s.repository.DeleteConversationMembers(ctx, tx, conversationId)
	if err != nil {
		return err
	}
	return s.publish(ctx, tx, conversationId, scheduledNotificationTopic, "",
		common.Marshal(common.NotificationScheduling{
			PartitionId: conversationId,
			Type:        "cancel-all",
		}))
}

func (s *service) BanParticipant(ctx context.Context, modId, conversationId, banId uuid.UUID) error {
	ok, err := s.repository.AddBanIdIfModerator(ctx, s.repository.Tx(), conversationId, modId, banId)
	if err != nil {
		return err
	}
	if !ok {
		err = errors.New("can't ban participant")
		slog.Warn("ban failed, ui error, or api abuse attempt",
			"conversationId", conversationId,
			"modId", modId,
			"banId", banId)
		return err
	}
	return nil
}

func (s *service) GetConversationDetail(ctx context.Context, conversationId, memberId uuid.UUID) (*dto.OnlineConversationDetailResponse, error) {
	detail, err := s.repository.FindConversationDetail(ctx, s.repository.Tx(), conversationId, memberId)
	if err != nil {
		return nil, err
	}
	canEnter := true
	if time.Now().UTC().Before(detail.Time.Add(-15 * time.Minute)) {
		canEnter = false
	}
	if time.Now().UTC().Before(detail.Time.Add(10*time.Minute)) && !detail.IsRegistrant {
		canEnter = false
	}
	if detail.IsBanned {
		canEnter = false
	}
	var updatedAt time.Time // zero, so omitted from the response, until the first update
	if detail.UpdatedAt != nil {
		updatedAt = *detail.UpdatedAt
	}
	resp := dto.OnlineConversationDetailResponse{
		Novel:                   detail.Novel,
		ShortStory:              detail.ShortStory,
		Poem:                    detail.Poem,
		Play:                    detail.Play,
		Film:                    detail.Film,
		WrittenBy:               detail.WrittenBy,
		Description:             detail.Description,
		Capacity:                detail.Capacity,
		Time:                    detail.Time,
		LengthMinutes:           detail.LengthMinutes,
		UpdatedAt:               updatedAt,
		CanEnter:                canEnter,
		IsModerator:             detail.IsModerator,
		IsRegistrant:            detail.IsRegistrant,
		IsNotificationScheduled: detail.IsNotificationScheduled,
	}
	return &resp, nil
}

func (s *service) RegisterConversation(ctx context.Context, memberId, conversationId uuid.UUID) error {
	tx, err := s.repository.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	registered, err := s.repository.TryIncrementRegistrants(ctx, tx, conversationId)
	if err != nil {
		return err
	}
	if !registered {
		return errors.New("already fully registered")
	}
	err = s.repository.InsertRegistrant(ctx, tx, conversationId, memberId)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		slog.Error("fail to commit transaction for register conversation", "err", err)
		return err
	}
	return nil
}

func (s *service) DeregisterConversation(ctx context.Context, memberId, conversationId uuid.UUID) error {
	tx, err := s.repository.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = s.repository.RemoveRegistrantId(ctx, tx, conversationId, memberId)
	if err != nil {
		return err
	}

	err = s.repository.DecrementRegistrants(ctx, tx, conversationId)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		slog.Error("fail to commit transaction for deregister conversation", "err", err)
		return err
	}
	return nil
}

func (s *service) ScheduleNotification(ctx context.Context, memberId, conversationId uuid.UUID) error {
	tx, err := s.repository.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	detail, err := s.repository.FindConversationDetail(ctx, tx, conversationId, memberId)
	if err != nil {
		return err
	}
	if !time.Now().UTC().Before(detail.Time.Add(-notificationTimeMinusInterval)) {
		return ErrTooLateForNotification
	}
	if err = s.repository.AddNotificationId(ctx, tx, conversationId, memberId); err != nil {
		return err
	}
	err = s.publish(ctx, tx, conversationId, scheduledNotificationTopic, "", common.Marshal(common.NotificationScheduling{
		PartitionId:   conversationId,
		KeyId:         memberId,
		ScheduledTime: detail.Time.Add(-notificationTimeMinusInterval).UnixMilli(),
		Contents:      map[int]string{0: about(detail)},
		Type:          "online-conversation",
	}))
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		slog.Error("fail to commit transaction for schedule notification", "err", err)
		return err
	}
	return nil
}

func (s *service) CancelNotification(ctx context.Context, memberId, conversationId uuid.UUID) error {
	tx, err := s.repository.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	err = s.repository.RemoveNotificationId(ctx, tx, conversationId, memberId)
	if err != nil {
		return err
	}
	err = s.publish(ctx, tx, conversationId, scheduledNotificationTopic, "",
		common.Marshal(common.NotificationScheduling{
			PartitionId: conversationId,
			KeyId:       memberId,
			Type:        "cancel",
		}))
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		slog.Error("fail to commit transaction for cancel notification", "err", err)
		return err
	}
	return nil
}

func (s *service) ReportConversation(ctx context.Context, conversationId, memberId uuid.UUID) error {
	err := s.producer.Commit("online-conversation", conversationId[:], nil, nil)
	if err != nil {
		return err
	}
	return nil
}

func (s *service) ManageReport(ctx context.Context, conversationId uuid.UUID) error {
	target, err := s.repository.FindReportTarget(ctx, s.repository.Tx(), conversationId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if target.IsEvaluated {
		return nil
	}

	verdict, err := s.moderation.Evaluate(ctx, target.Contents)
	if err != nil {
		return err
	}

	tx, err := s.repository.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	marked, err := s.repository.MarkEvaluatedIfUnchanged(ctx, tx, conversationId, target.UpdatedAt)
	if err != nil {
		return err
	}
	if !marked {
		slog.Info("conversation changed during report evaluation",
			"conversationId", conversationId,
			"violation", verdict.Violation)
		if !verdict.Violation {
			return errors.New("conversation contents was okay, but contents changed during report evaluation")
		}
		//so when it "was" a violation, we just delete the conversation even when it is updated while LLM work
	}
	err = s.repository.InsertVerdict(ctx, tx, conversationId, target, s.moderation.Model(), verdict)
	if err != nil {
		return err
	}
	if verdict.Violation {
		slog.Info("delete violating conversation",
			"conversationId", conversationId,
			"category", verdict.Category)
		err = s.repository.DeleteConversation(ctx, tx, conversationId)
		if err != nil {
			return err
		}
		err = s.removeConversationRest(ctx, tx, conversationId)
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		slog.Error("fail to commit transaction for report", "err", err)
		return err
	}
	return nil
}
