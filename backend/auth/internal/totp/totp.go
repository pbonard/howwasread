// Package totp holds the time based one time passwords (RFC 6238) of authenticator apps.
package totp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const (
	period = 30 // seconds a code lives, what authenticator apps assume
	// skew is the steps around now a code is accepted from, phones' clocks drift
	skew = 1
)

var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// timeStep is the time step of t that a code belongs to
func timeStep(t time.Time) int64 {
	return t.Unix() / period
}

// generateCode returns the 6 digit code of a base32 secret at a time step
func generateCode(secret string, step int64) (string, error) {
	key, err := encoding.DecodeString(secret)
	if err != nil {
		return "", err
	}
	counter := make([]byte, 8)
	binary.BigEndian.PutUint64(counter, uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(counter)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1000000), nil
}

// MatchStep returns the step within skew of now that the code belongs to, later than after so a used code can't be
// used again, and false when there is none
func MatchStep(secret, code string, now time.Time, after int64) (int64, bool, error) {
	current := timeStep(now)
	for step := current - skew; step <= current+skew; step++ {
		if step <= after {
			continue
		}
		expected, err := generateCode(secret, step)
		if err != nil {
			return 0, false, err
		}
		if subtle.ConstantTimeCompare([]byte(code), []byte(expected)) == 1 {
			return step, true, nil
		}
	}
	return 0, false, nil
}

// Encrypt seals a secret to its owner, so a leak of the database doesn't give the codes of every member and a secret
// copied to another member's row doesn't open
func Encrypt(key []byte, secret, email string) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	_, err = rand.Read(nonce)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(secret), []byte(email))), nil
}

// Decrypt opens what Encrypt sealed for the same email
func Decrypt(key []byte, sealed, email string) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("sealed totp secret is too short")
	}
	secret, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], []byte(email))
	if err != nil {
		return "", err
	}
	return string(secret), nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
