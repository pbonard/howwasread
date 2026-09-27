package test

import (
	"backend/auth/internal/service"
	"context"
	"testing"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const email = "alice@example.com"

func hash(t *testing.T, password string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return string(h)
}

func TestCreateMemberByEmail_rejectsBadInput(t *testing.T) {
	for name, in := range map[string][2]string{
		"invalid email":  {"not-an-email", "password123"},
		"short password": {email, "short"},
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := newService(t)

			_, err := s.CreateMemberByEmail(context.Background(), in[0], in[1])

			assert.ErrorIs(t, err, service.ErrSignUpWithEmail)
		})
	}
}

func TestCreateMemberByEmail_existingEmailIsRejected(t *testing.T) {
	s, d := newService(t)
	d.repo.EXPECT().VerifiedEmailExists(mock.Anything, email).Return(true, nil)

	_, err := s.CreateMemberByEmail(context.Background(), email, "password123")

	assert.ErrorIs(t, err, service.ErrSignUpWithEmail)
}

func TestCreateMemberByEmail_savesHashedPasswordAndMailsOTP(t *testing.T) {
	s, d := newService(t)
	d.repo.EXPECT().VerifiedEmailExists(mock.Anything, email).Return(false, nil)
	d.repo.EXPECT().SaveEmailLoginInfo(mock.Anything, email, mock.MatchedBy(func(hashed string) bool {
		return bcrypt.CompareHashAndPassword([]byte(hashed), []byte("password123")) == nil
	})).Return(nil)
	var savedOTP string
	var verificationId gocql.UUID
	d.repo.EXPECT().SaveEmailAndOtpByVerificationId(mock.Anything, email, mock.Anything).
		Run(func(vid gocql.UUID, _ string, otp string) { verificationId, savedOTP = vid, otp }).Return(nil)
	sent := expectMail(t, d.mailer, email)

	resp, err := s.CreateMemberByEmail(context.Background(), email, "password123")

	require.NoError(t, err)
	assert.Equal(t, uuid.UUID(verificationId), resp["verificationId"])
	assert.Regexp(t, `^\d{6}$`, savedOTP)
	assert.Equal(t, savedOTP, waitMail(t, sent))
}

func TestLoginWithEmail(t *testing.T) {
	id := uuid.New()
	t.Run("unknown account", func(t *testing.T) {
		s, d := newService(t)
		d.repo.EXPECT().FindLoginInfoByEmail(email).Return(false, false, gocql.UUID{}, "", "", gocql.ErrNotFound)

		_, _, err := s.LoginWithEmail(email, "password123")

		assert.EqualError(t, err, "this account does not exist")
	})
	t.Run("wrong password", func(t *testing.T) {
		s, d := newService(t)
		d.repo.EXPECT().FindLoginInfoByEmail(email).Return(true, true, gid(id), hash(t, "password123"), "user", nil)

		_, _, err := s.LoginWithEmail(email, "wrong-password")

		assert.ErrorIs(t, err, service.ErrLoginWithEmail)
	})
	t.Run("unverified email gets a new OTP", func(t *testing.T) {
		s, d := newService(t)
		d.repo.EXPECT().FindLoginInfoByEmail(email).Return(false, false, gid(id), hash(t, "password123"), "user", nil)
		d.repo.EXPECT().SaveEmailAndOtpByVerificationId(mock.Anything, email, mock.Anything).Return(nil)
		sent := expectMail(t, d.mailer, email)

		resp, rt, err := s.LoginWithEmail(email, "password123")

		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, resp.VerificationId)
		assert.Empty(t, resp.AccessToken)
		assert.Empty(t, rt)
		waitMail(t, sent)
	})
	t.Run("unverified phone gets a session to link it", func(t *testing.T) {
		s, d := newService(t)
		d.repo.EXPECT().FindLoginInfoByEmail(email).Return(true, false, gid(id), hash(t, "password123"), "user", nil)
		d.repo.EXPECT().SaveEmailBySessionId(mock.Anything, email).Return(nil)

		resp, rt, err := s.LoginWithEmail(email, "password123")

		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, resp.SessionId)
		assert.Empty(t, rt)
	})
	t.Run("verified account gets tokens", func(t *testing.T) {
		s, d := newService(t)
		d.repo.EXPECT().FindLoginInfoByEmail(email).Return(true, true, gid(id), hash(t, "password123"), "user", nil)
		var savedJTI gocql.UUID
		d.repo.EXPECT().SaveRefreshTokenJTIById(gid(id), mock.Anything).
			Run(func(_ gocql.UUID, jti gocql.UUID) { savedJTI = jti }).Return(nil)

		resp, rt, err := s.LoginWithEmail(email, "password123")

		require.NoError(t, err)
		at := claims(t, resp.AccessToken, keyAT)
		assert.Equal(t, id.String(), at["sub"])
		assert.Equal(t, "user", at["role"])
		assert.Equal(t, savedJTI.String(), claims(t, rt, keyRT)["jti"])
	})
}

func TestVerifyEmailOTP(t *testing.T) {
	vid := uuid.New()
	t.Run("wrong code", func(t *testing.T) {
		s, d := newService(t)
		d.repo.EXPECT().FindEmailAndOTPByVerificationId(gid(vid)).Return(email, "123456", nil)

		_, err := s.VerifyEmailOTP("654321", vid)

		assert.ErrorIs(t, err, service.ErrVerifyEmailOTP)
	})
	t.Run("correct code verifies the email and opens a session", func(t *testing.T) {
		s, d := newService(t)
		d.repo.EXPECT().FindEmailAndOTPByVerificationId(gid(vid)).Return(email, "123456", nil)
		d.repo.EXPECT().MarkEmailVerified(email).Return(nil)
		var sessionId gocql.UUID
		d.repo.EXPECT().SaveEmailBySessionId(mock.Anything, email).
			Run(func(sid gocql.UUID, _ string) { sessionId = sid }).Return(nil)

		resp, err := s.VerifyEmailOTP("123456", vid)

		require.NoError(t, err)
		assert.Equal(t, uuid.UUID(sessionId), resp.SessionId)
	})
}

func TestSetNewPassword_storesAHash(t *testing.T) {
	s, d := newService(t)
	sid := uuid.New()
	d.repo.EXPECT().FindEmailBySessionId(gid(sid)).Return(email, nil)
	d.repo.EXPECT().UpdatePasswordByEmail(mock.Anything, mock.MatchedBy(func(hashed string) bool {
		return bcrypt.CompareHashAndPassword([]byte(hashed), []byte("new-password")) == nil
	}), email).Return(nil)

	assert.NoError(t, s.SetNewPassword(context.Background(), "new-password", sid))
}
