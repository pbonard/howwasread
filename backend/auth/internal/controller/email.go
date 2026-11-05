package controller

import (
	"backend/auth/internal/constant"
	"backend/auth/internal/dto"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"
)

func emailRouter(n *Controller) {
	n.Router(POST, "/auth/email/apple", n.signInWithApple)
	n.Router(POST, "/auth/email/google", n.signInWithGoogle)
	if os.Getenv("NO_PASSWORD_EMAIL_LOGIN") == "true" {
		n.Router(POST, "/auth/email/login/start", n.startNoPasswordEmailLogin)
		n.Router(POST, "/auth/email/login/otp/verify", n.verifyNoPasswordEmailOTP)
		n.Router(POST, "/auth/email/login/totp/confirm", n.confirmTOTPSetup)
		n.Router(POST, "/auth/email/login/totp/verify", n.loginWithTOTP)
		n.Router(POST, "/auth/email/login/reset/start", n.startAuthenticatorReset)
		return
	}
	n.Router(POST, "/auth/email/create", n.createMemberByEmail)
	n.Router(POST, "/auth/email/login", n.loginWithEmail)
	n.Router(POST, "/auth/email/otp/verify", n.verifyEmailOTP)
	n.Router(POST, "/auth/email/password/forget", n.forgetPassword)
	n.Router(PATCH, "/auth/email/password/set-new", n.setNewPassword)
}

func (c *Controller) startNoPasswordEmailLogin(w http.ResponseWriter, r *http.Request) {
	var req dto.StartNoPasswordEmailLoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		handleError(w, err)
		return
	}
	result, err := c.service.StartNoPasswordEmailLogin(r.Context(), req.Email)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		slog.Error("fail to write response body",
			"err", err)
	}
}

func (c *Controller) verifyNoPasswordEmailOTP(w http.ResponseWriter, r *http.Request) {
	var req dto.VerifyEmailOTPRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		handleError(w, err)
		return
	}
	result, err := c.service.VerifyNoPasswordEmailOTP(r.Context(), req.VerificationId, req.OTP)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		slog.Error("fail to write response body",
			"err", err)
	}
}

func (c *Controller) confirmTOTPSetup(w http.ResponseWriter, r *http.Request) {
	var req dto.ConfirmTOTPSetupRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		handleError(w, err)
		return
	}
	result, rt, err := c.service.ConfirmTOTPSetup(r.Context(), req.SetupToken, req.Code)
	if err != nil {
		handleError(w, err)
		return
	}
	writeSignedIn(w, result, rt)
}

func (c *Controller) loginWithTOTP(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginWithTOTPRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		handleError(w, err)
		return
	}
	result, rt, err := c.service.LoginWithTOTP(r.Context(), req.Email, req.Code)
	if err != nil {
		handleError(w, err)
		return
	}
	writeSignedIn(w, result, rt)
}

func (c *Controller) startAuthenticatorReset(w http.ResponseWriter, r *http.Request) {
	var req dto.StartAuthenticatorResetRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		handleError(w, err)
		return
	}
	result, err := c.service.StartAuthenticatorReset(r.Context(), req.Email)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		slog.Error("fail to write response body",
			"err", err)
	}
}

func writeSignedIn(w http.ResponseWriter, result any, rt string) {
	if rt != "" {
		http.SetCookie(w, &http.Cookie{Name: "refresh_token",
			Value:    rt,
			Expires:  time.Now().Add(constant.RefreshTokenTTL * time.Second),
			Path:     "/",
			Domain:   "",
			HttpOnly: true,
			Secure:   true,
		})
	}
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(result)
	if err != nil {
		slog.Error("fail to write response body",
			"err", err)
	}
}

func (c *Controller) createMemberByEmail(w http.ResponseWriter, r *http.Request) {
	var req dto.SignInWithEmailRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		handleError(w, err)
		return
	}
	result, err := c.service.CreateMemberByEmail(r.Context(), req.Email, req.Password)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		slog.Error("fail to write response body",
			"err", err)
	}
}

func (c *Controller) loginWithEmail(w http.ResponseWriter, r *http.Request) {
	var req dto.SignInWithEmailRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		handleError(w, err)
		return
	}
	result, rt, err := c.service.LoginWithEmail(req.Email, req.Password)
	if err != nil {
		handleError(w, err)
		return
	}
	if rt != "" {
		http.SetCookie(w, &http.Cookie{Name: "refresh_token",
			Value:    rt,
			Expires:  time.Now().Add(constant.RefreshTokenTTL * time.Second),
			Path:     "/",
			Domain:   "",
			HttpOnly: true,
			Secure:   true,
		})
	}
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		slog.Error("fail to write response body",
			"err", err)
	}
}

func (c *Controller) verifyEmailOTP(w http.ResponseWriter, r *http.Request) {
	var req dto.VerifyEmailOTPRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		handleError(w, err)
		return
	}
	result, err := c.service.VerifyEmailOTP(req.OTP, req.VerificationId)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		slog.Error("fail to write response body",
			"err", err)
	}
}

func (c *Controller) signInWithApple(w http.ResponseWriter, r *http.Request) {
	var req dto.SignInWithAppleRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		handleError(w, err)
		return
	}
	responseBody, rt, err := c.service.SignInWithApple(
		r.Context(),
		req.IdentityToken,
	)
	if err != nil {
		handleError(w, err)
		return
	}
	if rt != "" {
		http.SetCookie(w, &http.Cookie{Name: "refresh_token",
			Value:    rt,
			Expires:  time.Now().Add(constant.RefreshTokenTTL * time.Second),
			Path:     "/",
			Domain:   "",
			HttpOnly: true,
			Secure:   true,
		})
	}
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(responseBody)
	if err != nil {
		slog.Error("fail to write response body",
			"err", err)
	}
}

func (c *Controller) signInWithGoogle(w http.ResponseWriter, r *http.Request) {
	var req dto.SignInWithGoogleRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		handleError(w, err)
		return
	}
	responseBody, rt, err := c.service.SignInWithGoogle(r.Context(), req.IdToken)
	if err != nil {
		handleError(w, err)
		return
	}
	if rt != "" {
		http.SetCookie(w, &http.Cookie{Name: "refresh_token",
			Value:    rt,
			Expires:  time.Now().Add(constant.RefreshTokenTTL * time.Second),
			Path:     "/",
			Domain:   "",
			HttpOnly: true,
			Secure:   true,
		})
	}
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(responseBody)
	if err != nil {
		slog.Error("fail to write response body",
			"err", err)
	}
}

func (c *Controller) forgetPassword(w http.ResponseWriter, r *http.Request) {
	var req dto.ForgetPasswordRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		handleError(w, err)
		return
	}
	result, err := c.service.ForgetPassword(r.Context(), req.Email)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		slog.Error("fail to write response body",
			"err", err)
	}
}

func (c *Controller) setNewPassword(w http.ResponseWriter, r *http.Request) {
	var req dto.SetNewPasswordRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		handleError(w, err)
		return
	}
	err = c.service.SetNewPassword(r.Context(), req.Password, req.SessionId)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
