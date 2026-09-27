package repository

import (
	"backend/onlineconversation/internal/dto"
	"backend/onlineconversation/internal/projection"
	"context"
	"log/slog"

	"github.com/google/uuid"
)

func (r *repository) InsertConversation(ctx context.Context, session Session, conversationId uuid.UUID, req dto.CreateConversationRequest) error {
	_, err := session.ExecContext(ctx, `
		INSERT INTO online_conversation
		(id, novel, short_story, poem, play, film, written_by, rule, capacity,
		time, length_minutes, current_registrants)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
		conversationId[:], req.Novel, req.ShortStory, req.Poem, req.Play, req.Film,
		req.WrittenBy, req.Rule,
		req.Capacity, req.Time, req.LengthMinutes)
	if err != nil {
		slog.Error("fail to insert new online conversation", "err", err)
		return err
	}
	return nil
}

func (r *repository) UpdateConversationIfModerator(ctx context.Context, session Session, memberId uuid.UUID, req dto.UpdateConversationRequest) (bool, error) {
	res, err := session.ExecContext(ctx, `
		UPDATE online_conversation
		SET novel=?, short_story=?, poem=?, play=?, film=?,
		written_by=?, rule=?, capacity=?, time=?, length_minutes=?, updated_at=UTC_TIMESTAMP()
		WHERE id=?
		AND EXISTS (SELECT 1 FROM online_conversation_moderator
		WHERE conversation_id=? AND member_id=?)`,
		req.Novel, req.ShortStory, req.Poem, req.Play, req.Film,
		req.WrittenBy, req.Rule, req.Capacity, req.Time, req.LengthMinutes,
		req.Id[:], req.Id[:], memberId[:])
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (r *repository) DeleteOnlineConversationIfModerator(ctx context.Context, session Session, conversationId, memberId uuid.UUID) (bool, error) {
	res, err := session.ExecContext(ctx, `
		DELETE FROM online_conversation
		WHERE id = ?
		AND EXISTS (SELECT 1 FROM online_conversation_moderator
		WHERE conversation_id=? AND member_id=?)`,
		conversationId[:], conversationId[:], memberId[:])
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (r *repository) AddBanIdIfModerator(ctx context.Context, session Session, conversationId, modId, banId uuid.UUID) (bool, error) {
	res, err := session.ExecContext(ctx, `
		INSERT INTO online_conversation_ban (conversation_id, member_id)
		SELECT ?, ?
		WHERE EXISTS (SELECT 1 FROM online_conversation_moderator
		WHERE conversation_id=? AND member_id=?)`,
		conversationId[:], banId[:],
		conversationId[:], modId[:])
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (r *repository) InsertModerator(ctx context.Context, session Session, conversationId, memberId uuid.UUID) error {
	_, err := session.ExecContext(ctx,
		`INSERT IGNORE INTO online_conversation_moderator (conversation_id, member_id) VALUES (?, ?)`,
		conversationId[:], memberId[:],
	)
	if err != nil {
		slog.Error("fail to insert moderator",
			"conversationId", conversationId, "memberId", memberId, "err", err)
		return err
	}
	return nil
}

func (r *repository) InsertRegistrant(ctx context.Context, session Session, conversationId, memberId uuid.UUID) error {
	_, err := session.ExecContext(ctx,
		`INSERT IGNORE INTO online_conversation_registrant (conversation_id, member_id)
		VALUES (?, ?)`,
		conversationId[:], memberId[:],
	)
	if err != nil {
		slog.Error("fail to insert registrant",
			"conversationId", conversationId, "memberId", memberId, "err", err)
		return err
	}
	return nil
}

func (r *repository) AddNotificationId(ctx context.Context, session Session, conversationId, memberId uuid.UUID) error {
	_, err := session.ExecContext(ctx,
		`INSERT IGNORE INTO online_conversation_notification
       (conversation_id, member_id) VALUES (?, ?)`,
		conversationId[:], memberId[:])
	if err != nil {
		slog.Error("fail to add notification id to online conversation",
			"conversationId", conversationId, "memberId", memberId, "err", err)
		return err
	}
	return nil
}

func (r *repository) FindConversationDetail(ctx context.Context, session Session, conversationId, memberId uuid.UUID) (d projection.Detail, err error) {
	row := session.QueryRowContext(ctx, `
		SELECT novel, short_story, poem, play, film, written_by, rule, capacity,
		time, length_minutes, updated_at,
		EXISTS(SELECT 1 FROM online_conversation_moderator
		WHERE conversation_id = c.id AND member_id = ?),
		EXISTS(SELECT 1 FROM online_conversation_registrant
		WHERE conversation_id = c.id AND member_id = ?),
		EXISTS(SELECT 1 FROM online_conversation_ban
		WHERE conversation_id = c.id AND member_id = ?),
		EXISTS(SELECT 1 FROM online_conversation_notification
		WHERE conversation_id = c.id AND member_id = ?)
		FROM online_conversation c
		WHERE c.id = ?`,
		memberId[:], memberId[:], memberId[:], memberId[:], conversationId[:],
	)
	err = row.Scan(
		&d.Novel, &d.ShortStory, &d.Poem, &d.Play, &d.Film, &d.WrittenBy, &d.Rule, &d.Capacity,
		&d.Time, &d.LengthMinutes, &d.UpdatedAt,
		&d.IsModerator, &d.IsRegistrant, &d.IsBanned, &d.IsNotificationScheduled)
	if err != nil {
		slog.Error("fail to find online conversation detail", "err", err)
		return projection.Detail{}, err
	}
	return d, nil
}

func (r *repository) TryIncrementRegistrants(ctx context.Context, session Session, conversationId uuid.UUID) (bool, error) {
	result, err := session.ExecContext(ctx,
		`UPDATE online_conversation SET
		current_registrants = current_registrants + 1
		WHERE id = ? AND current_registrants < capacity`,
		conversationId[:],
	)
	if err != nil {
		slog.Error("fail to increment online conversation registrants",
			"conversationId", conversationId, "err", err)
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *repository) DecrementRegistrants(ctx context.Context, session Session, conversationId uuid.UUID) error {
	_, err := session.ExecContext(ctx,
		`UPDATE online_conversation SET
		current_registrants = current_registrants - 1 WHERE id = ?`,
		conversationId[:],
	)
	if err != nil {
		slog.Error("fail to decrement online conversation registrants",
			"conversationId", conversationId, "err", err)
		return err
	}
	return nil
}

func (r *repository) RemoveRegistrantId(ctx context.Context, session Session, conversationId, memberId uuid.UUID) error {
	_, err := session.ExecContext(ctx,
		`DELETE FROM online_conversation_registrant
       WHERE conversation_id = ? AND member_id = ?`,
		conversationId[:], memberId[:],
	)
	if err != nil {
		slog.Error("fail to remove online conversation registrant id",
			"conversationId", conversationId, "memberId", memberId, "err", err)
		return err
	}
	return nil
}

func (r *repository) FindNotificationIds(ctx context.Context, session Session, conversationId uuid.UUID) ([]uuid.UUID, error) {
	rows, err := session.QueryContext(ctx,
		`SELECT member_id FROM online_conversation_notification WHERE conversation_id = ?`,
		conversationId[:])
	if err != nil {
		slog.Error("fail to find online conversation notification ids", "err", err, "conversationId", conversationId)
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		id, err := uuid.FromBytes(raw)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *repository) RemoveNotificationId(ctx context.Context, session Session, conversationId, memberId uuid.UUID) error {
	_, err := session.ExecContext(ctx,
		`DELETE FROM online_conversation_notification
       WHERE conversation_id = ? AND member_id = ?`,
		conversationId[:], memberId[:])
	if err != nil {
		slog.Error("fail to remove online conversation notification id",
			"err", err,
			"conversationId", conversationId,
			"memberId", memberId)
		return err
	}
	return nil
}
