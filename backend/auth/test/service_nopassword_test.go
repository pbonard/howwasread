package test

import (
	"backend/auth/internal/constant"
	"backend/auth/internal/repository"
	"backend/auth/internal/service"
	"backend/auth/internal/totp"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var testTOTPKey = make([]byte, 32)

func newNoPasswordService(t *testing.T) (service.Service, deps) {
	t.Setenv("NO_PASSWORD_EMAIL_LOGIN", "true")
	t.Setenv("TOTP_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(testTOTPKey))
	t.Setenv("PHONE_VERIFICATION_REQUIRED", "false")
	t.Setenv("ISSUER", "Holidayfriend")
	return newService(t)
}

func sealedSecret(t *testing.T) (secret, sealed string) {
	secret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	sealed, err := totp.Encrypt(testTOTPKey, secret, email)
	require.NoError(t, err)
	return secret, sealed
}

func currentCode(t *testing.T, secret string) (string, int64) {
	step := time.Now().Unix() / 30
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	require.NoError(t, err)
	mac := hmac.New(sha1.New, key)
	mac.Write(binary.BigEndian.AppendUint64(nil, uint64(step)))
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1000000), step
}

func TestStartNoPasswordEmailLogin(t *testing.T) {
	t.Run("invalid email", func(t *testing.T) {
		s, _ := newNoPasswordService(t)

		_, err := s.StartNoPasswordEmailLogin(context.Background(), "not-an-email")

		assert.ErrorIs(t, err, service.ErrSendEmailOTP)
	})
	t.Run("a member with an authenticator gets no email", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return("sealed", int64(1), nil)

		res, err := s.StartNoPasswordEmailLogin(context.Background(), email)

		require.NoError(t, err)
		assert.False(t, res.IsSignup)
		assert.Equal(t, uuid.Nil, res.VerificationId)
	})
	t.Run("a second request inside the cooldown sends nothing", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return("", int64(0), gocql.ErrNotFound)
		d.repo.EXPECT().FindLoginInfoByEmail(email).Return(false, false, gocql.UUID{}, "", "", gocql.ErrNotFound)
		d.repo.EXPECT().ClaimNoPasswordEmailOTPCooldown(mock.Anything, email).Return(repository.ErrAlreadyExists)

		_, err := s.StartNoPasswordEmailLogin(context.Background(), email)

		assert.ErrorIs(t, err, service.ErrEmailOTPTooSoon)
	})
	t.Run("a failed lookup doesn't fall back to email", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return("", int64(0), errors.New("db down"))

		_, err := s.StartNoPasswordEmailLogin(context.Background(), email)

		assert.ErrorIs(t, err, service.ErrInternalServer)
	})
	t.Run("a google or apple member gets no email and no setup", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return("", int64(0), gocql.ErrNotFound)
		d.repo.EXPECT().FindLoginInfoByEmail(email).Return(true, false, gocql.UUID{}, "", constant.RoleUser, nil)

		_, err := s.StartNoPasswordEmailLogin(context.Background(), email)

		assert.ErrorIs(t, err, service.ErrUseThirdPartySignIn)
	})
	t.Run("a failed member lookup doesn't send a code", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return("", int64(0), gocql.ErrNotFound)
		d.repo.EXPECT().FindLoginInfoByEmail(email).Return(false, false, gocql.UUID{}, "", "", errors.New("db down"))

		_, err := s.StartNoPasswordEmailLogin(context.Background(), email)

		assert.ErrorIs(t, err, service.ErrInternalServer)
	})
	t.Run("saves and mails a 6 digit code for the normalized email", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		sent := make(chan string, 1)
		var saved string
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return("", int64(0), gocql.ErrNotFound)
		d.repo.EXPECT().FindLoginInfoByEmail(email).Return(false, false, gocql.UUID{}, "", "", gocql.ErrNotFound)
		d.repo.EXPECT().ClaimNoPasswordEmailOTPCooldown(mock.Anything, email).Return(nil)
		d.repo.EXPECT().SaveNoPasswordEmailOTP(mock.Anything, mock.Anything, email, mock.Anything, constant.OTPPurposeSignup).
			RunAndReturn(func(_ context.Context, _ gocql.UUID, _, otp, _ string) error {
				saved = otp
				return nil
			})
		d.mailer.EXPECT().SendOTP(email, mock.Anything).RunAndReturn(func(_, otp string) error {
			sent <- otp
			return nil
		})

		res, err := s.StartNoPasswordEmailLogin(context.Background(), "  ALICE@example.com ")

		require.NoError(t, err)
		assert.True(t, res.IsSignup)
		assert.NotEqual(t, uuid.Nil, res.VerificationId)
		assert.Regexp(t, `^\d{6}$`, saved)
		assert.Equal(t, saved, <-sent)
	})
}

func TestVerifyNoPasswordEmailOTP(t *testing.T) {
	vid := uuid.New()
	const code = "123456"

	t.Run("unknown or expired code", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindNoPasswordEmailOTP(mock.Anything, gid(vid)).Return("", "", "", 0, gocql.ErrNotFound)

		_, err := s.VerifyNoPasswordEmailOTP(context.Background(), vid, code)

		assert.ErrorIs(t, err, service.ErrVerifyEmailOTP)
	})
	t.Run("wrong code is counted", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindNoPasswordEmailOTP(mock.Anything, gid(vid)).Return(email, code, constant.OTPPurposeSignup, 2, nil)
		d.repo.EXPECT().AddNoPasswordEmailOTPAttempt(mock.Anything, gid(vid), 2).Return(nil)

		_, err := s.VerifyNoPasswordEmailOTP(context.Background(), vid, "000000")

		assert.ErrorIs(t, err, service.ErrVerifyEmailOTP)
	})
	t.Run("the right code stops working after too many attempts", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindNoPasswordEmailOTP(mock.Anything, gid(vid)).Return(email, code, constant.OTPPurposeSignup, constant.NoPasswordEmailOTPMaxAttempts, nil)

		_, err := s.VerifyNoPasswordEmailOTP(context.Background(), vid, code)

		assert.ErrorIs(t, err, service.ErrVerifyEmailOTP)
	})
	t.Run("an expired row with only attempts left never matches an empty code", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindNoPasswordEmailOTP(mock.Anything, gid(vid)).Return("", "", "", 1, nil)

		_, err := s.VerifyNoPasswordEmailOTP(context.Background(), vid, "")

		assert.ErrorIs(t, err, service.ErrVerifyEmailOTP)
	})
	t.Run("the right code hands out a setup, and no sign in", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		var stored string
		d.repo.EXPECT().FindNoPasswordEmailOTP(mock.Anything, gid(vid)).Return(email, code, constant.OTPPurposeSignup, 0, nil)
		d.repo.EXPECT().AddNoPasswordEmailOTPAttempt(mock.Anything, gid(vid), 0).Return(nil)
		d.repo.EXPECT().DeleteNoPasswordEmailOTP(mock.Anything, gid(vid)).Return(nil)
		d.repo.EXPECT().SaveTOTPSetup(mock.Anything, mock.Anything, email, mock.Anything, constant.OTPPurposeSignup).
			RunAndReturn(func(_ context.Context, _ gocql.UUID, _, sealed, _ string) error {
				stored = sealed
				return nil
			})

		res, err := s.VerifyNoPasswordEmailOTP(context.Background(), vid, code)

		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, res.SetupToken)
		assert.Len(t, res.Secret, 32, "20 bytes in base32")
		assert.Contains(t, res.OtpauthURL, "otpauth://totp/Holidayfriend:alice@example.com")
		assert.Contains(t, res.OtpauthURL, "issuer=Holidayfriend")
		assert.Contains(t, res.OtpauthURL, "secret="+res.Secret)
		png, err := base64.StdEncoding.DecodeString(res.QRCode)
		require.NoError(t, err)
		assert.Equal(t, "\x89PNG", string(png[:4]))
		assert.NotContains(t, stored, res.Secret, "the secret is stored encrypted")
	})
}

func TestConfirmTOTPSetup(t *testing.T) {
	token := uuid.New()
	secret, sealed := sealedSecret(t)

	t.Run("expired setup", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindTOTPSetup(mock.Anything, gid(token)).Return("", "", "", gocql.ErrNotFound)

		_, _, err := s.ConfirmTOTPSetup(context.Background(), token, "123456")

		assert.ErrorIs(t, err, service.ErrVerifyTOTP)
	})
	t.Run("a wrong code doesn't turn it on or sign in", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindTOTPSetup(mock.Anything, gid(token)).Return(email, sealed, constant.OTPPurposeSignup, nil)
		d.repo.EXPECT().FindTOTPAttempts(mock.Anything, email).Return(0, nil)
		d.repo.EXPECT().AddTOTPAttempt(mock.Anything, email, 0).Return(nil)

		_, rt, err := s.ConfirmTOTPSetup(context.Background(), token, "not-a-code")

		assert.ErrorIs(t, err, service.ErrVerifyTOTP)
		assert.Empty(t, rt)
	})
	t.Run("a setup can't replace a working authenticator", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		code, step := currentCode(t, secret)
		d.repo.EXPECT().FindTOTPSetup(mock.Anything, gid(token)).Return(email, sealed, constant.OTPPurposeSignup, nil)
		d.repo.EXPECT().FindTOTPAttempts(mock.Anything, email).Return(0, nil)
		d.repo.EXPECT().AddTOTPAttempt(mock.Anything, email, 0).Return(nil)
		d.repo.EXPECT().EnableTOTP(mock.Anything, email, sealed, step).Return(repository.ErrAlreadyExists)

		_, rt, err := s.ConfirmTOTPSetup(context.Background(), token, code)

		assert.ErrorIs(t, err, service.ErrVerifyTOTP)
		assert.Empty(t, rt)
	})
	t.Run("the right code turns it on and signs up", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		code, step := currentCode(t, secret)
		d.repo.EXPECT().FindTOTPSetup(mock.Anything, gid(token)).Return(email, sealed, constant.OTPPurposeSignup, nil)
		d.repo.EXPECT().FindTOTPAttempts(mock.Anything, email).Return(0, nil)
		d.repo.EXPECT().AddTOTPAttempt(mock.Anything, email, 0).Return(nil)
		d.repo.EXPECT().EnableTOTP(mock.Anything, email, sealed, step).Return(nil)
		d.repo.EXPECT().DeleteTOTPSetup(mock.Anything, gid(token)).Return(nil)
		d.repo.EXPECT().DeleteTOTPAttempts(mock.Anything, email).Return(nil)
		d.repo.EXPECT().FindLoginInfoByEmail(email).Return(false, false, gocql.UUID{}, "", "", gocql.ErrNotFound)
		d.repo.EXPECT().SaveThirdPartySignInInfo(mock.Anything, mock.Anything, email, false, true).Return(nil)
		d.repo.EXPECT().SaveProfileId(mock.Anything).Return(nil)
		d.repo.EXPECT().SaveRefreshTokenJTIById(mock.Anything, mock.Anything).Return(nil)

		res, rt, err := s.ConfirmTOTPSetup(context.Background(), token, code)

		require.NoError(t, err)
		assert.NotEmpty(t, res.AccessToken)
		assert.NotEmpty(t, rt)
	})
}

func TestLoginWithTOTP(t *testing.T) {
	id := uuid.New()
	secret, sealed := sealedSecret(t)

	signIn := func(d deps) {
		d.repo.EXPECT().DeleteTOTPAttempts(mock.Anything, email).Return(nil)
		d.repo.EXPECT().FindLoginInfoByEmail(email).Return(true, false, gid(id), "", "user", nil)
		d.repo.EXPECT().SaveThirdPartySignInInfo(mock.Anything, gid(id), email, false, true).Return(nil)
		d.repo.EXPECT().SaveProfileId(gid(id)).Return(nil)
		d.repo.EXPECT().SaveRefreshTokenJTIById(gid(id), mock.Anything).Return(nil)
	}

	t.Run("an email without an authenticator can't log in here", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return("", int64(0), gocql.ErrNotFound)

		_, _, err := s.LoginWithTOTP(context.Background(), email, "123456")

		assert.ErrorIs(t, err, service.ErrVerifyTOTP)
	})
	t.Run("too many attempts lock even the right code out", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		code, _ := currentCode(t, secret)
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return(sealed, int64(0), nil)
		d.repo.EXPECT().FindTOTPAttempts(mock.Anything, email).Return(constant.TOTPMaxAttempts, nil)

		_, _, err := s.LoginWithTOTP(context.Background(), email, code)

		assert.ErrorIs(t, err, service.ErrTOTPTooManyAttempts)
	})
	t.Run("a guess that loses the race for an attempt is rejected", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		code, _ := currentCode(t, secret)
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return(sealed, int64(0), nil)
		d.repo.EXPECT().FindTOTPAttempts(mock.Anything, email).Return(1, nil)
		d.repo.EXPECT().AddTOTPAttempt(mock.Anything, email, 1).Return(repository.ErrAlreadyExists)

		_, _, err := s.LoginWithTOTP(context.Background(), email, code)

		assert.ErrorIs(t, err, service.ErrVerifyTOTP)
	})
	t.Run("a wrong code doesn't sign in", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return(sealed, int64(0), nil)
		d.repo.EXPECT().FindTOTPAttempts(mock.Anything, email).Return(0, nil)
		d.repo.EXPECT().AddTOTPAttempt(mock.Anything, email, 0).Return(nil)

		_, rt, err := s.LoginWithTOTP(context.Background(), email, "wrong")

		assert.ErrorIs(t, err, service.ErrVerifyTOTP)
		assert.Empty(t, rt)
	})
	t.Run("the code just used can't be used again", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		code, step := currentCode(t, secret)
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return(sealed, step+1, nil)
		d.repo.EXPECT().FindTOTPAttempts(mock.Anything, email).Return(0, nil)
		d.repo.EXPECT().AddTOTPAttempt(mock.Anything, email, 0).Return(nil)

		_, _, err := s.LoginWithTOTP(context.Background(), email, code)

		assert.ErrorIs(t, err, service.ErrVerifyTOTP)
	})
	t.Run("a code used by a concurrent request is rejected", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		code, step := currentCode(t, secret)
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return(sealed, int64(0), nil)
		d.repo.EXPECT().FindTOTPAttempts(mock.Anything, email).Return(0, nil)
		d.repo.EXPECT().AddTOTPAttempt(mock.Anything, email, 0).Return(nil)
		d.repo.EXPECT().MarkTOTPStep(mock.Anything, email, int64(0), step).Return(repository.ErrAlreadyExists)

		_, _, err := s.LoginWithTOTP(context.Background(), email, code)

		assert.ErrorIs(t, err, service.ErrVerifyTOTP)
	})
	t.Run("a secret copied from another member doesn't open", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		copied, err := totp.Encrypt(testTOTPKey, secret, "mallory@example.com")
		require.NoError(t, err)
		code, _ := currentCode(t, secret)
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return(copied, int64(0), nil)
		d.repo.EXPECT().FindTOTPAttempts(mock.Anything, email).Return(0, nil)
		d.repo.EXPECT().AddTOTPAttempt(mock.Anything, email, 0).Return(nil)

		_, _, err = s.LoginWithTOTP(context.Background(), email, code)

		assert.ErrorIs(t, err, service.ErrInternalServer)
	})
	t.Run("the right code signs in", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		code, step := currentCode(t, secret)
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return(sealed, int64(0), nil)
		d.repo.EXPECT().FindTOTPAttempts(mock.Anything, email).Return(0, nil)
		d.repo.EXPECT().AddTOTPAttempt(mock.Anything, email, 0).Return(nil)
		d.repo.EXPECT().MarkTOTPStep(mock.Anything, email, int64(0), step).Return(nil)
		signIn(d)

		resp, rt, err := s.LoginWithTOTP(context.Background(), "ALICE@example.com", code)

		require.NoError(t, err)
		assert.Equal(t, id.String(), claims(t, resp.AccessToken, keyAT)["sub"])
		assert.NotEmpty(t, rt)
	})
}

func TestStartAuthenticatorRecovery(t *testing.T) {
	t.Run("an email without an authenticator gets no code", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return("", int64(0), gocql.ErrNotFound)

		_, err := s.StartAuthenticatorRecovery(context.Background(), email)

		assert.ErrorIs(t, err, service.ErrSendEmailOTP)
	})
	t.Run("a second request inside the cooldown sends nothing", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return("sealed", int64(1), nil)
		d.repo.EXPECT().ClaimNoPasswordEmailOTPCooldown(mock.Anything, email).Return(repository.ErrAlreadyExists)

		_, err := s.StartAuthenticatorRecovery(context.Background(), email)

		assert.ErrorIs(t, err, service.ErrEmailOTPTooSoon)
	})
	t.Run("mails a code for recovery and leaves the authenticator alone", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindTOTP(mock.Anything, email).Return("sealed", int64(1), nil)
		d.repo.EXPECT().ClaimNoPasswordEmailOTPCooldown(mock.Anything, email).Return(nil)
		d.repo.EXPECT().SaveNoPasswordEmailOTP(mock.Anything, mock.Anything, email, mock.Anything, constant.OTPPurposeRecovery).Return(nil)
		sent := expectMail(t, d.mailer, email)

		res, err := s.StartAuthenticatorRecovery(context.Background(), "  ALICE@example.com ")

		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, res.VerificationId)
		assert.Regexp(t, `^\d{6}$`, waitMail(t, sent))
	})
}

func TestVerifyNoPasswordEmailOTP_recovery(t *testing.T) {
	vid := uuid.New()
	const code = "123456"

	t.Run("the setup is for what the code was mailed for, and no sign in", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindNoPasswordEmailOTP(mock.Anything, gid(vid)).Return(email, code, constant.OTPPurposeRecovery, 0, nil)
		d.repo.EXPECT().AddNoPasswordEmailOTPAttempt(mock.Anything, gid(vid), 0).Return(nil)
		d.repo.EXPECT().DeleteNoPasswordEmailOTP(mock.Anything, gid(vid)).Return(nil)
		d.repo.EXPECT().SaveTOTPSetup(mock.Anything, mock.Anything, email, mock.Anything, constant.OTPPurposeRecovery).Return(nil)

		res, err := s.VerifyNoPasswordEmailOTP(context.Background(), vid, code)

		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, res.SetupToken)
	})
}

func TestConfirmTOTPSetup_recovery(t *testing.T) {
	id := uuid.New()
	token := uuid.New()
	secret, sealed := sealedSecret(t)

	t.Run("a member without an authenticator has nothing to replace", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		code, step := currentCode(t, secret)
		d.repo.EXPECT().FindTOTPSetup(mock.Anything, gid(token)).Return(email, sealed, constant.OTPPurposeRecovery, nil)
		d.repo.EXPECT().FindTOTPAttempts(mock.Anything, email).Return(0, nil)
		d.repo.EXPECT().AddTOTPAttempt(mock.Anything, email, 0).Return(nil)
		d.repo.EXPECT().ReplaceTOTP(mock.Anything, email, sealed, step).Return(repository.ErrAlreadyExists)

		_, rt, err := s.ConfirmTOTPSetup(context.Background(), token, code)

		assert.ErrorIs(t, err, service.ErrVerifyTOTP)
		assert.Empty(t, rt)
	})
	t.Run("a wrong code keeps the old authenticator", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		d.repo.EXPECT().FindTOTPSetup(mock.Anything, gid(token)).Return(email, sealed, constant.OTPPurposeRecovery, nil)
		d.repo.EXPECT().FindTOTPAttempts(mock.Anything, email).Return(0, nil)
		d.repo.EXPECT().AddTOTPAttempt(mock.Anything, email, 0).Return(nil)

		_, rt, err := s.ConfirmTOTPSetup(context.Background(), token, "not-a-code")

		assert.ErrorIs(t, err, service.ErrVerifyTOTP)
		assert.Empty(t, rt)
	})
	t.Run("sessions that can't be ended stop the sign in", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		code, step := currentCode(t, secret)
		d.repo.EXPECT().FindTOTPSetup(mock.Anything, gid(token)).Return(email, sealed, constant.OTPPurposeRecovery, nil)
		d.repo.EXPECT().FindTOTPAttempts(mock.Anything, email).Return(0, nil)
		d.repo.EXPECT().AddTOTPAttempt(mock.Anything, email, 0).Return(nil)
		d.repo.EXPECT().ReplaceTOTP(mock.Anything, email, sealed, step).Return(nil)
		d.repo.EXPECT().FindLoginInfoByEmail(email).Return(true, false, gid(id), "", "user", nil)
		d.repo.EXPECT().ClearRefreshTokenJTIsById(mock.Anything, gid(id)).Return(errors.New("db down"))

		_, rt, err := s.ConfirmTOTPSetup(context.Background(), token, code)

		assert.ErrorIs(t, err, service.ErrInternalServer)
		assert.Empty(t, rt)
	})
	t.Run("the right code replaces it, ends other sessions, tells the owner and signs in", func(t *testing.T) {
		s, d := newNoPasswordService(t)
		code, step := currentCode(t, secret)
		told := make(chan string, 1)
		d.repo.EXPECT().FindTOTPSetup(mock.Anything, gid(token)).Return(email, sealed, constant.OTPPurposeRecovery, nil)
		d.repo.EXPECT().FindTOTPAttempts(mock.Anything, email).Return(0, nil)
		d.repo.EXPECT().AddTOTPAttempt(mock.Anything, email, 0).Return(nil)
		d.repo.EXPECT().ReplaceTOTP(mock.Anything, email, sealed, step).Return(nil)
		cleared := d.repo.EXPECT().ClearRefreshTokenJTIsById(mock.Anything, gid(id)).Return(nil).Call
		d.mailer.EXPECT().SendAuthenticatorChanged(email).Run(func(to string) { told <- to }).Return(nil)
		d.repo.EXPECT().DeleteTOTPSetup(mock.Anything, gid(token)).Return(nil)
		d.repo.EXPECT().DeleteTOTPAttempts(mock.Anything, email).Return(nil)
		d.repo.EXPECT().FindLoginInfoByEmail(email).Return(true, false, gid(id), "", "user", nil)
		d.repo.EXPECT().SaveThirdPartySignInInfo(mock.Anything, gid(id), email, false, true).Return(nil)
		d.repo.EXPECT().SaveProfileId(gid(id)).Return(nil)
		// the new session must survive the clearing
		d.repo.EXPECT().SaveRefreshTokenJTIById(gid(id), mock.Anything).NotBefore(cleared).Return(nil)

		resp, rt, err := s.ConfirmTOTPSetup(context.Background(), token, code)

		require.NoError(t, err)
		assert.Equal(t, id.String(), claims(t, resp.AccessToken, keyAT)["sub"])
		assert.NotEmpty(t, rt)
		assert.Equal(t, email, waitMail(t, told))
	})
}
