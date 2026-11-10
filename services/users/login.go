package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/acilerts/cloud-drive/internal/platform"
	"golang.org/x/crypto/bcrypt"
)

const maxOTPAttempts = 5

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type verifyRequest struct {
	Email string `json:"email"`
	OTP   string `json:"otp"`
}

func writeCoded(w http.ResponseWriter, status int, msg, code string) {
	platform.WriteJSON(w, status, map[string]string{"error": msg, "code": code})
}

func (s *Server) invalidCredentials(w http.ResponseWriter) {
	platform.WriteError(w, http.StatusUnauthorized, "invalid email or password")
}

func (s *Server) passwordExpired(u *User) bool {
	return time.Since(u.PasswordChangedAt) > s.passwordMaxAge
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.logger.Warn("validation failed", "endpoint", "login", "reason", "invalid_json")
		platform.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if validateEmail(email) != "" || req.Password == "" || len(req.Password) > 72 {
		s.logger.Warn("validation failed", "endpoint", "login", "reason", "bad_format")
		platform.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	user, err := s.store.FindUserByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(req.Password))
		s.logger.Warn("login attempt", "success", false, "reason", "unknown_user")
		s.invalidCredentials(w)
		return
	}
	if err != nil {
		s.internalError(w, "find user failed", err)
		return
	}

	if time.Now().Before(user.LockedUntil) {
		s.logger.Warn("login attempt", "success", false, "reason", "account_locked", "userId", user.ID.Hex())
		writeCoded(w, http.StatusTooManyRequests, "too many failed attempts, try again later", "account_locked")
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		locked, err := s.store.RecordFailedLogin(ctx, user.ID, s.maxFailures, s.lockoutFor)
		if err != nil {
			s.logger.Error("record failed login failed", "error", err)
		}
		s.logger.Warn("login attempt", "success", false, "reason", "wrong_password",
			"userId", user.ID.Hex(), "locked", locked)
		s.invalidCredentials(w)
		return
	}

	if err := s.store.ClearFailedLogins(ctx, user.ID); err != nil {
		s.logger.Error("clear failed logins failed", "error", err)
	}
	if user.Status != StatusActive {
		s.logger.Warn("login attempt", "success", false, "reason", "not_confirmed", "userId", user.ID.Hex())
		writeCoded(w, http.StatusForbidden, "account not confirmed, check your email", "account_not_confirmed")
		return
	}
	if s.passwordExpired(user) {
		s.logger.Warn("login attempt", "success", false, "reason", "password_expired", "userId", user.ID.Hex())
		writeCoded(w, http.StatusForbidden, "password expired, reset it by email", "password_expired")
		return
	}

	code, err := newOTP()
	if err == nil {
		err = s.store.SaveOTP(ctx, user.ID, hashOTP(user.ID, code), s.otpTTL)
	}
	if err != nil {
		s.internalError(w, "create otp failed", err)
		return
	}
	body := "Zdravo " + user.FirstName + ",\n\nVaš jednokratni kod za prijavu je: " + code +
		"\n\nKod važi " + s.otpTTL.String() + " i može se iskoristiti samo jednom.\n"
	if err := s.mailer.Send(user.Email, "Kod za prijavu", body); err != nil {
		s.logger.Error("sending otp mail failed", "error", err)
		platform.WriteError(w, http.StatusServiceUnavailable, "could not send login code")
		return
	}

	s.logger.Info("login step 1 passed, otp sent", "userId", user.ID.Hex())
	platform.WriteJSON(w, http.StatusOK, map[string]string{"message": "login code sent to your email"})
}

func validOTP(s string) bool {
	if len(s) != 6 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (s *Server) handleLoginVerify(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.logger.Warn("validation failed", "endpoint", "login_verify", "reason", "invalid_json")
		platform.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if validateEmail(email) != "" || !validOTP(req.OTP) {
		s.logger.Warn("validation failed", "endpoint", "login_verify", "reason", "bad_format")
		platform.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	user, err := s.store.FindUserByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		s.logger.Warn("login attempt", "success", false, "reason", "otp_unknown_user")
		platform.WriteError(w, http.StatusUnauthorized, "invalid or expired code")
		return
	}
	if err != nil {
		s.internalError(w, "find user failed", err)
		return
	}

	err = s.store.VerifyOTP(ctx, user.ID, hashOTP(user.ID, req.OTP), maxOTPAttempts)
	if errors.Is(err, ErrInvalidToken) {
		s.logger.Warn("login attempt", "success", false, "reason", "wrong_otp", "userId", user.ID.Hex())
		platform.WriteError(w, http.StatusUnauthorized, "invalid or expired code")
		return
	}
	if err != nil {
		s.internalError(w, "verify otp failed", err)
		return
	}

	if user.Status != StatusActive || s.passwordExpired(user) {
		s.logger.Warn("login attempt", "success", false, "reason", "state_changed", "userId", user.ID.Hex())
		platform.WriteError(w, http.StatusForbidden, "login not allowed")
		return
	}

	token, err := platform.SignToken(s.jwtKey, user.ID.Hex(), user.Role, s.jwtTTL)
	if err != nil {
		s.internalError(w, "sign token failed", err)
		return
	}
	s.logger.Info("login attempt", "success", true, "userId", user.ID.Hex())
	platform.WriteJSON(w, http.StatusOK, map[string]any{
		"token":     token,
		"expiresIn": int(s.jwtTTL.Seconds()),
		"user": map[string]string{
			"id":        user.ID.Hex(),
			"firstName": user.FirstName,
			"lastName":  user.LastName,
			"email":     user.Email,
			"role":      user.Role,
		},
	})
}
