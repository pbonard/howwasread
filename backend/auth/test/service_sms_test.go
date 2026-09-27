package test

import (
	"backend/auth/internal/service"
	"errors"
	"testing"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const phone = "+821012345678"

func TestSendSMSOTP(t *testing.T) {
	t.Run("banned number", func(t *testing.T) {
		s, d := newService(t)
		d.repo.EXPECT().WasBanned(phone).Return(nil)

		_, err := s.SendSMSOTP(uuid.Nil, phone)

		assert.EqualError(t, err, "this phone number is not usable")
	})
	t.Run("sign in with a number linked to an email account", func(t *testing.T) {
		s, d := newService(t)
		d.repo.EXPECT().WasBanned(phone).Return(gocql.ErrNotFound)
		d.repo.EXPECT().FindEmailByPhoneNumber(phone).Return(email, nil)

		_, err := s.SendSMSOTP(uuid.Nil, phone)

		assert.ErrorIs(t, err, service.ErrPhoneNumberAlreadyLinked)
	})
	t.Run("sends the OTP and remembers the number", func(t *testing.T) {
		s, d := newService(t)
		d.repo.EXPECT().WasBanned(phone).Return(gocql.ErrNotFound)
		d.repo.EXPECT().FindEmailByPhoneNumber(phone).Return("", gocql.ErrNotFound)
		d.sms.EXPECT().SendOTP(phone).Return(phone, nil)
		var vid gocql.UUID
		d.repo.EXPECT().SavePhoneNumberByVerificationId(mock.Anything, phone).
			Run(func(v gocql.UUID, _ string) { vid = v }).Return(nil)

		resp, err := s.SendSMSOTP(uuid.Nil, phone)

		require.NoError(t, err)
		assert.Equal(t, uuid.UUID(vid), resp["verificationId"])
	})
	t.Run("twilio failure", func(t *testing.T) {
		s, d := newService(t)
		d.repo.EXPECT().WasBanned(phone).Return(gocql.ErrNotFound)
		d.sms.EXPECT().SendOTP(phone).Return("", errors.New("twilio down"))

		// with a session the linked-email check is skipped
		_, err := s.SendSMSOTP(uuid.New(), phone)

		assert.ErrorIs(t, err, service.ErrSendSMSOTP)
	})
}

func TestVerifySMSOTP(t *testing.T) {
	vid := uuid.New()
	t.Run("code not approved", func(t *testing.T) {
		s, d := newService(t)
		d.repo.EXPECT().FindPhoneNumberByVerificationId(gid(vid)).Return(phone, nil)
		d.sms.EXPECT().CheckOTP(phone, "123456").Return(false, nil)

		_, _, err := s.VerifySMSOTP(uuid.Nil, vid, "123456")

		assert.ErrorIs(t, err, service.ErrVerifySMSOTP)
	})
	t.Run("new phone sign in creates the member and logs in", func(t *testing.T) {
		s, d := newService(t)
		d.repo.EXPECT().FindPhoneNumberByVerificationId(gid(vid)).Return(phone, nil)
		d.sms.EXPECT().CheckOTP(phone, "123456").Return(true, nil)
		d.repo.EXPECT().FindIdByPhoneNumber(phone).Return(gocql.UUID{}, gocql.ErrNotFound)
		var newId gocql.UUID
		d.repo.EXPECT().SavePhoneNumberLoginInfo(phone, mock.Anything).
			Run(func(_ string, id gocql.UUID) { newId = id }).Return(nil)
		d.repo.EXPECT().SaveRefreshTokenJTIById(mock.Anything, mock.Anything).Return(nil)
		d.repo.EXPECT().SaveProfileId(mock.Anything).Return(nil)

		resp, rt, err := s.VerifySMSOTP(uuid.Nil, vid, "123456")

		require.NoError(t, err)
		assert.Equal(t, newId.String(), claims(t, resp.AccessToken, keyAT)["sub"])
		assert.Equal(t, newId.String(), claims(t, rt, keyRT)["sub"])
	})
	t.Run("email session links the phone to the email account", func(t *testing.T) {
		s, d := newService(t)
		sid, id := uuid.New(), uuid.New()
		d.repo.EXPECT().FindEmailBySessionId(gid(sid)).Return(email, nil)
		d.repo.EXPECT().FindPhoneNumberByVerificationId(gid(vid)).Return(phone, nil)
		d.repo.EXPECT().FindEmailByPhoneNumber(phone).Return("", gocql.ErrNotFound)
		d.sms.EXPECT().CheckOTP(phone, "123456").Return(true, nil)
		d.repo.EXPECT().FindLoginInfoByEmail(email).Return(true, false, gid(id), "", "user", nil)
		d.repo.EXPECT().FindIdByPhoneNumber(phone).Return(gocql.UUID{}, gocql.ErrNotFound)
		d.repo.EXPECT().LinkAndMarkVerifiedPhoneNumber(gid(id), email, phone, "user").Return(nil)
		d.repo.EXPECT().SaveRefreshTokenJTIById(gid(id), mock.Anything).Return(nil)
		d.repo.EXPECT().SaveProfileId(gid(id)).Return(nil)

		resp, _, err := s.VerifySMSOTP(sid, vid, "123456")

		require.NoError(t, err)
		assert.Equal(t, id.String(), claims(t, resp.AccessToken, keyAT)["sub"])
	})
}
