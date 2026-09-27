package test

import (
	"backend/auth/internal/constant"
	"backend/auth/internal/service"
	"context"
	"errors"
	"testing"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSignInWithGoogle(t *testing.T) {
	id := uuid.New()
	t.Run("invalid token", func(t *testing.T) {
		s, d := newService(t)
		d.google.EXPECT().Email(mock.Anything, "token").Return("", errors.New("bad token"))

		_, _, err := s.SignInWithGoogle(context.Background(), "token")

		assert.ErrorIs(t, err, service.ErrSignInWithGoogle)
	})
	t.Run("new user gets a session to verify the phone", func(t *testing.T) {
		s, d := newService(t)
		d.google.EXPECT().Email(mock.Anything, "token").Return(email, nil)
		d.repo.EXPECT().FindLoginInfoByEmail(email).Return(false, false, gocql.UUID{}, "", "", gocql.ErrNotFound)
		d.repo.EXPECT().SaveThirdPartySignInInfo(mock.Anything, mock.Anything, email, false, true).Return(nil)
		d.repo.EXPECT().SaveEmailBySessionId(mock.Anything, email).Return(nil)

		resp, rt, err := s.SignInWithGoogle(context.Background(), "token")

		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, resp.SessionId)
		assert.Empty(t, rt)
	})
	t.Run("verified member gets tokens", func(t *testing.T) {
		s, d := newService(t)
		d.google.EXPECT().Email(mock.Anything, "token").Return(email, nil)
		d.repo.EXPECT().FindLoginInfoByEmail(email).Return(true, true, gid(id), "", "user", nil)
		d.repo.EXPECT().SaveThirdPartySignInInfo(mock.Anything, gid(id), email, true, true).Return(nil)
		d.repo.EXPECT().SaveRefreshTokenJTIById(gid(id), mock.Anything).Return(nil)

		resp, rt, err := s.SignInWithGoogle(context.Background(), "token")

		require.NoError(t, err)
		assert.Equal(t, id.String(), claims(t, resp.AccessToken, keyAT)["sub"])
		assert.NotEmpty(t, rt)
	})
}

func appleToken(t *testing.T, override jwt.MapClaims) string {
	c := jwt.MapClaims{
		"iss":   constant.AppleIssuerUrl,
		"aud":   "com.example.app",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"nonce": "nonce-1",
		"email": email,
	}
	for k, v := range override {
		c[k] = v
	}
	return sign(t, keyApple, c)
}

func TestSignInWithApple_rejectsBadTokens(t *testing.T) {
	tests := map[string]string{
		"wrong issuer":   appleToken(t, jwt.MapClaims{"iss": "https://evil.example.com"}),
		"wrong audience": appleToken(t, jwt.MapClaims{"aud": "com.other.app"}),
		"expired":        appleToken(t, jwt.MapClaims{"exp": time.Now().Add(-time.Minute).Unix()}),
		"no nonce":       appleToken(t, jwt.MapClaims{"nonce": nil}),
		"other key":      sign(t, keyOther, jwt.MapClaims{"iss": constant.AppleIssuerUrl}),
	}
	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			s, _ := newService(t)

			_, _, err := s.SignInWithApple(context.Background(), token)

			assert.ErrorIs(t, err, service.ErrSignInWithApple)
		})
	}
}

func TestSignInWithApple_replayedNonceIsRejected(t *testing.T) {
	s, d := newService(t)
	d.repo.EXPECT().CheckNonce("nonce-1").Return(true, nil)

	_, _, err := s.SignInWithApple(context.Background(), appleToken(t, nil))

	assert.ErrorIs(t, err, service.ErrSignInWithApple)
}

func TestSignInWithApple_verifiedMemberGetsTokens(t *testing.T) {
	s, d := newService(t)
	id := uuid.New()
	d.repo.EXPECT().CheckNonce("nonce-1").Return(false, nil)
	d.repo.EXPECT().SaveNonce("nonce-1").Return(nil)
	d.repo.EXPECT().FindLoginInfoByEmail(email).Return(true, true, gid(id), "", "user", nil)
	d.repo.EXPECT().SaveThirdPartySignInInfo(mock.Anything, gid(id), email, true, true).Return(nil)
	d.repo.EXPECT().SaveRefreshTokenJTIById(gid(id), mock.Anything).Return(nil)

	resp, rt, err := s.SignInWithApple(context.Background(), appleToken(t, nil))

	require.NoError(t, err)
	assert.Equal(t, id.String(), claims(t, resp.AccessToken, keyAT)["sub"])
	assert.NotEmpty(t, rt)
}
