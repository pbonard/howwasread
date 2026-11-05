package repository

import (
	"backend/auth/internal/constant"
	"context"
	"log/slog"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
)

func (r *repository) FindRefreshTokenJTIsById(id gocql.UUID) (jtis []gocql.UUID, err error) {
	err = r.session.Query(
		"SELECT refresh_token_jtis FROM member_by_id WHERE id = ?",
		id,
	).Scan(&jtis)
	if err != nil {
		slog.Info("fail to get refresh token jti",
			"err", err,
		)
		return nil, err
	}
	return jtis, nil
}

func (r *repository) SaveRefreshTokenJTIById(id, jti gocql.UUID) error {
	err := r.session.Query(
		"UPDATE member_by_id USING TTL ? SET refresh_token_jtis += ? WHERE id = ?",
		constant.RefreshTokenTTL, []gocql.UUID{jti}, id,
	).Exec()
	if err != nil {
		slog.Error("fail to save refresh token jti",
			"err", err,
		)
		return err
	}
	return nil
}

func (r *repository) RemoveRefreshTokenJTIById(id, jti gocql.UUID) error {
	err := r.session.Query(
		"UPDATE member_by_id SET refresh_token_jtis -= ? WHERE id = ?",
		[]gocql.UUID{jti}, id,
	).Exec()
	if err != nil {
		slog.Error("fail to save refresh token jti",
			"err", err,
		)
		return err
	}
	return nil
}

// ClearRefreshTokenJTIsById ends every session of a member
func (r *repository) ClearRefreshTokenJTIsById(ctx context.Context, id gocql.UUID) error {
	err := r.session.Query("DELETE refresh_token_jtis FROM member_by_id WHERE id = ?", id).ExecContext(ctx)
	if err != nil {
		slog.Error("fail to clear refresh token jtis",
			"err", err,
		)
		return err
	}
	return nil
}

func (r *repository) FindEmailAndPhoneNumberById(ctx context.Context, id gocql.UUID) (email, phoneNumber string, err error) {
	err = r.session.Query("SELECT email, phone_number FROM member_by_id WHERE id = ?", id).
		ScanContext(ctx, &email, &phoneNumber)
	if err != nil {
		slog.Error("fail to find email and phone number by id",
			"err", err,
			"id", id)
		return "", "", err
	}
	return email, phoneNumber, nil
}

func (r *repository) DeleteAccount(ctx context.Context, id gocql.UUID, email, phoneNumber string) error {
	batch := r.session.Batch(gocql.LoggedBatch)
	batch.Query("DELETE FROM member_by_id WHERE id = ?", id)
	batch.Query("DELETE FROM profile_by_id WHERE id = ?", id)
	if email != "" {
		batch.Query("DELETE FROM member_by_email WHERE email = ?", email)
	}
	if phoneNumber != "" {
		batch.Query("DELETE FROM member_by_phone_number WHERE phone_number = ?", phoneNumber)
	}
	err := batch.ExecContext(ctx)
	if err != nil {
		slog.Error("fail to delete account",
			"err", err,
			"id", id.String(),
		)
		return err
	}
	return nil
}
