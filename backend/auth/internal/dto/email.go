package dto

import "github.com/google/uuid"

type SignInWithEmailRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginWithEmailResponse struct {
	VerificationId uuid.UUID `json:"verificationId,omitzero"`
	SessionId      uuid.UUID `json:"sessionId,omitzero"`
	AccessToken    string    `json:"accessToken,omitempty"`
}

type VerifyEmailOTPRequest struct {
	VerificationId uuid.UUID `json:"verificationId"`
	OTP            string    `json:"otp"`
}
type VerifyEmailOTPResponse struct {
	SessionId uuid.UUID `json:"sessionId"`
}

type StartNoPasswordEmailLoginRequest struct {
	Email string `json:"email"`
}

type StartNoPasswordEmailLoginResponse struct {
	IsSignup       bool      `json:"isSignup"`
	VerificationId uuid.UUID `json:"verificationId,omitzero"`
}

type TOTPSetupResponse struct {
	SetupToken uuid.UUID `json:"setupToken"`
	// a base64 encoded PNG
	QRCode     string `json:"qrCode"`
	Secret     string `json:"secret"`
	OtpauthURL string `json:"otpauthUrl"`
}

type ConfirmTOTPSetupRequest struct {
	SetupToken uuid.UUID `json:"setupToken"`
	Code       string    `json:"code"`
}

type LoginWithTOTPRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type StartAuthenticatorResetRequest struct {
	Email string `json:"email"`
}

type StartAuthenticatorResetResponse struct {
	VerificationId uuid.UUID `json:"verificationId"`
}

type SignInWithAppleRequest struct {
	IdentityToken string `json:"identityToken"`
}

type SignInWithGoogleRequest struct {
	IdToken string `json:"idToken"`
}

type SignInWithThirdPartyResponse struct {
	SessionId   uuid.UUID `json:"sessionId,omitzero"`
	AccessToken string    `json:"accessToken,omitempty"`
}

type ForgetPasswordRequest struct {
	Email string `json:"email"`
}

type SetNewPasswordRequest struct {
	Password  string    `json:"password"`
	SessionId uuid.UUID `json:"sessionId"`
}
