package client

import (
	"context"
	"errors"
	"log"
	"os"

	"google.golang.org/api/idtoken"
)

// GoogleAuthClient verifies Google sign in ID tokens
type GoogleAuthClient interface {
	// Email returns the email of a valid ID token issued for this app
	Email(ctx context.Context, idToken string) (string, error)
}

type googleAuthClient struct {
	validator *idtoken.Validator
	audience  string
}

func NewGoogleAuthClient() GoogleAuthClient {
	validator, err := idtoken.NewValidator(context.Background())
	if err != nil {
		log.Fatalf("Failed to create google id token validator: %v", err)
	}
	audience := os.Getenv("GOOGLE_SIGN_IN_WEB_CLIENT_ID")
	// the validator skips the audience check when it's empty, and would accept tokens issued for any app
	if audience == "" {
		log.Fatal("GOOGLE_SIGN_IN_WEB_CLIENT_ID is not set")
	}
	return &googleAuthClient{validator: validator, audience: audience}
}

func (g *googleAuthClient) Email(ctx context.Context, idToken string) (string, error) {
	payload, err := g.validator.Validate(ctx, idToken, g.audience)
	if err != nil {
		return "", err
	}
	return VerifiedEmail(payload.Claims)
}

// VerifiedEmail returns the email of the claims only if google verified it, an unverified one must not
// sign in to the member who owns that address
func VerifiedEmail(claims map[string]any) (string, error) {
	email, ok := claims["email"].(string)
	if !ok {
		return "", errors.New("no email in google id token")
	}
	verified, _ := claims["email_verified"].(bool)
	if !verified {
		return "", errors.New("email in google id token is not verified")
	}
	return email, nil
}
