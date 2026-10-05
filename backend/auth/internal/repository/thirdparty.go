package repository

import (
	"backend/auth/internal/constant"
	"context"
	"log/slog"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
)

// SaveNonce claims the nonce once, a nonce already saved by a concurrent request returns ErrAlreadyExists
func (r *repository) SaveNonce(nonce string) error {
	applied, err := r.session.Query(
		`INSERT INTO nonce (nonce) VALUES (?) IF NOT EXISTS USING TTL ?`, nonce, constant.NonceTTL,
	).MapScanCAS(map[string]any{})
	if err != nil {
		slog.Error("fail to insert nonce",
			"err", err,
			"nonce", nonce)
		return err
	}
	if !applied {
		slog.Info("nonce is already used", "nonce", nonce)
		return ErrAlreadyExists
	}
	return nil
}

// SaveThirdPartySignInInfo claims member_by_email before writing member_by_id, so a sign in that loses a race
// to another member with the same email returns ErrAlreadyExists instead of overwriting it
func (r *repository) SaveThirdPartySignInInfo(ctx context.Context, id gocql.UUID, email string, phoneNumberVerified, emailVerified bool) error {
	var applied bool
	var err error
	existing := map[string]any{}
	if emailVerified {
		// a new member, or the verified member itself whose row is left as it is
		applied, err = r.session.Query(
			`INSERT INTO member_by_email (
                             email_verified, phone_number_verified, id, email, role
                             ) VALUES (?, ?, ?, ?, ?) IF NOT EXISTS`,
			emailVerified, phoneNumberVerified, id, email, constant.RoleUser).MapScanCASContext(ctx, existing)
		if err == nil && !applied && existing["id"] == id {
			applied = true
		}
	} else {
		// the third party proves the email, so it takes over an email sign up which is not verified yet
		applied, err = r.session.Query(
			`UPDATE member_by_email SET email_verified = ?, password = ?, phone_number_verified = ?, id = ?, role = ?
                             WHERE email = ? IF email_verified = ?`,
			true, nil, phoneNumberVerified, id, constant.RoleUser, email, false).MapScanCASContext(ctx, existing)
	}
	if err != nil {
		slog.Error("fail to save third party sign in info at member_by_email",
			"err", err,
			"id", id.String(),
		)
		return err
	}
	if !applied {
		slog.Info("email is already claimed by another member", "id", id.String())
		return ErrAlreadyExists
	}
	err = r.session.Query(
		`INSERT INTO member_by_id (
                          email_verified, phone_number_verified, id, email, role
                          ) VALUES (?, ?, ?, ?, ?)`,
		true, phoneNumberVerified, id, email, constant.RoleUser).
		ExecContext(ctx)
	if err != nil {
		slog.Error("fail to save third party sign in info at member_by_id",
			"err", err,
			"id", id.String(),
		)
		return err
	}
	return nil
}
