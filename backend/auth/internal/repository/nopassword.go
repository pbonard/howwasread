package repository

import (
	"backend/auth/internal/constant"
	"context"
	"errors"
	"log/slog"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
)

// ClaimNoPasswordEmailOTPCooldown returns ErrAlreadyExists when a code was already requested for the email within the cooldown
func (r *repository) ClaimNoPasswordEmailOTPCooldown(ctx context.Context, email string) error {
	applied, err := r.session.Query(
		`INSERT INTO email_otp_cooldown (email) VALUES (?) IF NOT EXISTS USING TTL ?`,
		email, constant.NoPasswordEmailOTPCooldownTTL,
	).MapScanCASContext(ctx, map[string]any{})
	if err != nil {
		slog.Error("fail to claim email otp cooldown", "err", err)
		return err
	}
	if !applied {
		return ErrAlreadyExists
	}
	return nil
}

func (r *repository) SaveNoPasswordEmailOTP(ctx context.Context, verificationId gocql.UUID, email, otp, purpose string) error {
	err := r.session.Query(
		`INSERT INTO email_otp (verification_id, email, otp, purpose, attempts) VALUES (?, ?, ?, ?, 0) USING TTL ?`,
		verificationId, email, otp, purpose, constant.OtpTTL,
	).ExecContext(ctx)
	if err != nil {
		slog.Error("fail to save email otp", "err", err)
		return err
	}
	return nil
}

func (r *repository) FindNoPasswordEmailOTP(ctx context.Context, verificationId gocql.UUID) (email, otp, purpose string, attempts int, err error) {
	err = r.session.Query(
		`SELECT email, otp, purpose, attempts FROM email_otp WHERE verification_id = ?`,
		verificationId,
	).ScanContext(ctx, &email, &otp, &purpose, &attempts)
	if err != nil {
		slog.Info("fail to find email otp, might be expired", "err", err)
		return "", "", "", 0, err
	}
	return email, otp, purpose, attempts, nil
}

// AddNoPasswordEmailOTPAttempt counts a guess from the attempts just read, a guess that lost the race returns ErrAlreadyExists,
// so concurrent guesses can't share one attempt
func (r *repository) AddNoPasswordEmailOTPAttempt(ctx context.Context, verificationId gocql.UUID, attempts int) error {
	applied, err := r.session.Query(
		`UPDATE email_otp USING TTL ? SET attempts = ? WHERE verification_id = ? IF attempts = ?`,
		constant.OtpTTL, attempts+1, verificationId, attempts,
	).MapScanCASContext(ctx, map[string]any{})
	if err != nil {
		slog.Error("fail to add email otp attempt", "err", err)
		return err
	}
	if !applied {
		return ErrAlreadyExists
	}
	return nil
}

func (r *repository) DeleteNoPasswordEmailOTP(ctx context.Context, verificationId gocql.UUID) error {
	err := r.session.Query(
		`DELETE FROM email_otp WHERE verification_id = ?`, verificationId,
	).ExecContext(ctx)
	if err != nil {
		slog.Error("fail to delete email otp", "err", err)
		return err
	}
	return nil
}

// FindTOTP returns the authenticator of a member, gocql.ErrNotFound when it has none
func (r *repository) FindTOTP(ctx context.Context, email string) (sealedSecret string, lastStep int64, err error) {
	err = r.session.Query(
		`SELECT secret, last_step FROM member_totp WHERE email = ?`, email,
	).ScanContext(ctx, &sealedSecret, &lastStep)
	if err != nil {
		if !errors.Is(err, gocql.ErrNotFound) {
			slog.Error("fail to find totp", "err", err)
		}
		return "", 0, err
	}
	return sealedSecret, lastStep, nil
}

func (r *repository) SaveTOTPSetup(ctx context.Context, token gocql.UUID, email, sealedSecret, purpose string) error {
	err := r.session.Query(
		`INSERT INTO totp_setup (setup_token, email, secret, purpose) VALUES (?, ?, ?, ?) USING TTL ?`,
		token, email, sealedSecret, purpose, constant.TOTPSetupTTL,
	).ExecContext(ctx)
	if err != nil {
		slog.Error("fail to save totp setup", "err", err)
		return err
	}
	return nil
}

func (r *repository) FindTOTPSetup(ctx context.Context, token gocql.UUID) (email, sealedSecret, purpose string, err error) {
	err = r.session.Query(
		`SELECT email, secret, purpose FROM totp_setup WHERE setup_token = ?`, token,
	).ScanContext(ctx, &email, &sealedSecret, &purpose)
	if err != nil {
		slog.Info("fail to find totp setup, might be expired", "err", err)
		return "", "", "", err
	}
	return email, sealedSecret, purpose, nil
}

func (r *repository) DeleteTOTPSetup(ctx context.Context, token gocql.UUID) error {
	err := r.session.Query(`DELETE FROM totp_setup WHERE setup_token = ?`, token).ExecContext(ctx)
	if err != nil {
		slog.Error("fail to delete totp setup", "err", err)
		return err
	}
	return nil
}

// EnableTOTP returns ErrAlreadyExists when the member has one, so a setup can't replace a working authenticator
func (r *repository) EnableTOTP(ctx context.Context, email, sealedSecret string, step int64) error {
	applied, err := r.session.Query(
		`INSERT INTO member_totp (email, secret, last_step) VALUES (?, ?, ?) IF NOT EXISTS`,
		email, sealedSecret, step,
	).MapScanCASContext(ctx, map[string]any{})
	if err != nil {
		slog.Error("fail to enable totp", "err", err)
		return err
	}
	if !applied {
		return ErrAlreadyExists
	}
	return nil
}

// ReplaceTOTP swaps the authenticator of a member for a new one, ErrAlreadyExists when the member has none to replace
func (r *repository) ReplaceTOTP(ctx context.Context, email, sealedSecret string, step int64) error {
	applied, err := r.session.Query(
		`UPDATE member_totp SET secret = ?, last_step = ? WHERE email = ? IF EXISTS`, sealedSecret, step, email,
	).MapScanCASContext(ctx, map[string]any{})
	if err != nil {
		slog.Error("fail to replace totp", "err", err)
		return err
	}
	if !applied {
		return ErrAlreadyExists
	}
	return nil
}

// MarkTOTPStep marks a code as used, a code used by a concurrent request returns ErrAlreadyExists
func (r *repository) MarkTOTPStep(ctx context.Context, email string, lastStep, step int64) error {
	applied, err := r.session.Query(
		`UPDATE member_totp SET last_step = ? WHERE email = ? IF last_step = ?`, step, email, lastStep,
	).MapScanCASContext(ctx, map[string]any{})
	if err != nil {
		slog.Error("fail to mark totp step", "err", err)
		return err
	}
	if !applied {
		return ErrAlreadyExists
	}
	return nil
}

// FindTOTPAttempts returns the wrong guesses counted in the current window, 0 when there is none
func (r *repository) FindTOTPAttempts(ctx context.Context, email string) (int, error) {
	var attempts int
	err := r.session.Query(`SELECT attempts FROM totp_attempt WHERE email = ?`, email).ScanContext(ctx, &attempts)
	if errors.Is(err, gocql.ErrNotFound) {
		return 0, nil
	}
	if err != nil {
		slog.Error("fail to find totp attempts", "err", err)
		return 0, err
	}
	return attempts, nil
}

// AddTOTPAttempt counts a guess from the attempts just read, one that lost the race returns ErrAlreadyExists
func (r *repository) AddTOTPAttempt(ctx context.Context, email string, attempts int) error {
	query := r.session.Query(
		`UPDATE totp_attempt USING TTL ? SET attempts = ? WHERE email = ? IF attempts = ?`,
		constant.TOTPAttemptTTL, attempts+1, email, attempts,
	)
	if attempts == 0 {
		query = r.session.Query(
			`INSERT INTO totp_attempt (email, attempts) VALUES (?, 1) IF NOT EXISTS USING TTL ?`,
			email, constant.TOTPAttemptTTL,
		)
	}
	applied, err := query.MapScanCASContext(ctx, map[string]any{})
	if err != nil {
		slog.Error("fail to add totp attempt", "err", err)
		return err
	}
	if !applied {
		return ErrAlreadyExists
	}
	return nil
}

func (r *repository) DeleteTOTPAttempts(ctx context.Context, email string) error {
	err := r.session.Query(`DELETE FROM totp_attempt WHERE email = ?`, email).ExecContext(ctx)
	if err != nil {
		slog.Error("fail to delete totp attempts", "err", err)
		return err
	}
	return nil
}
