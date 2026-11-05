package totp

import (
	"encoding/base32"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rfcSecret = "12345678901234567890"

func TestTOTPCode_matchesTheRFC6238Vectors(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(rfcSecret))
	for unix, want := range map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"} {
		code, err := generateCode(secret, timeStep(time.Unix(unix, 0)))
		require.NoError(t, err)
		assert.Equal(t, want, code, "at %d", unix)
	}
}

func TestTOTPMatchStep(t *testing.T) {
	secret := encoding.EncodeToString([]byte(rfcSecret))
	now := time.Unix(1700000000, 0)
	code := func(step int64) string {
		c, err := generateCode(secret, step)
		require.NoError(t, err)
		return c
	}
	current := timeStep(now)

	t.Run("accepts one step of clock drift each way", func(t *testing.T) {
		for _, step := range []int64{current - 1, current, current + 1} {
			got, ok, err := MatchStep(secret, code(step), now, 0)
			require.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, step, got)
		}
	})
	t.Run("rejects a code from further away", func(t *testing.T) {
		_, ok, err := MatchStep(secret, code(current-2), now, 0)
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("rejects a code that was already used", func(t *testing.T) {
		_, ok, err := MatchStep(secret, code(current), now, current)
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

func TestTOTPEncryption(t *testing.T) {
	key := make([]byte, 32)
	other := make([]byte, 32)
	other[0] = 1

	sealed, err := Encrypt(key, "SECRET", "alice@example.com")
	require.NoError(t, err)
	assert.NotContains(t, sealed, "SECRET")

	opened, err := Decrypt(key, sealed, "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, "SECRET", opened)

	_, err = Decrypt(other, sealed, "alice@example.com")
	assert.Error(t, err)

	_, err = Decrypt(key, sealed, "mallory@example.com")
	assert.Error(t, err)
}
