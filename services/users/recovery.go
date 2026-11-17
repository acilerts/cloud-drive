package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/acilerts/cloud-drive/internal/platform"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
)

const (
	TokenReset = "reset"
	TokenMagic = "magic"
)

const genericLinkMessage = "if the account exists, an email has been sent"

func tooNew(changedAt time.Time, minAge time.Duration) bool {
	return time.Since(changedAt) < minAge
}

func sameAsCurrent(u *User, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
}

func checkNewPassword(pass, repeat, passField, repeatField string) map[string]string {
	errs := map[string]string{}
	if m := validatePassword(pass); m != "" {
		errs[passField] = m
	}
	if pass != repeat {
		errs[repeatField] = "Lozinke se ne poklapaju"
	}
	return errs
}

func (s *Server) writeFieldErrors(w http.ResponseWriter, endpoint string, fields map[string]string) {
	names := make([]string, 0, len(fields))
	for k := range fields {
		names = append(names, k)
	}
	s.logger.Warn("validation failed", "endpoint", endpoint, "fields", names)
	platform.WriteJSON(w, http.StatusBadRequest,
		map[string]any{"error": "validation failed", "fields": fields})
}

func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (*platform.Claims, bool) {
	raw, ok := platform.BearerToken(r)
	if !ok {
		s.logger.Warn("access denied", "reason", "missing_token", "path", r.URL.Path)
		platform.WriteError(w, http.StatusUnauthorized, "authentication required")
		return nil, false
	}
	/*
		claims, err := platform.VerifyToken(s.jwtPub, raw)
		if err != nil {
			s.logger.Warn("access denied", "reason", "invalid_or_expired_token", "path", r.URL.Path)
			platform.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
			return nil, false
		}*/

	claims, err := platform.VerifyToken(s.jwtPub, raw)
	if err != nil {
		s.logger.Warn("access denied",
			"reason", "invalid_or_expired_token",
			"path", r.URL.Path,
			"error", err,
		)
		platform.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
		return nil, false
	}
	return claims, true
}

type emailRequest struct {
	Email string `json:"email"`
}

func (s *Server) readEmail(w http.ResponseWriter, r *http.Request, endpoint string) (string, bool) {
	var req emailRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.logger.Warn("validation failed", "endpoint", endpoint, "reason", "invalid_json")
		platform.WriteError(w, http.StatusBadRequest, "invalid request body")
		return "", false
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if validateEmail(email) != "" {
		s.logger.Warn("validation failed", "endpoint", endpoint, "reason", "bad_email")
		platform.WriteError(w, http.StatusBadRequest, "invalid request body")
		return "", false
	}
	return email, true
}

func (s *Server) sendRecoveryLink(email, typ string, ttl time.Duration, page, subject, intro string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	user, err := s.store.FindUserByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		s.logger.Warn("recovery link requested", "type", typ, "result", "unknown_user")
		return
	}
	if err != nil {
		s.logger.Error("recovery: find user failed", "error", err)
		return
	}
	if user.Status != StatusActive {
		s.logger.Warn("recovery link requested", "type", typ, "result", "not_active", "userId", user.ID.Hex())
		return
	}

	raw, hash, err := newToken()
	if err == nil {
		err = s.store.ReplaceToken(ctx, user.ID, typ, hash, ttl)
	}
	if err != nil {
		s.logger.Error("recovery: create token failed", "error", err)
		return
	}

	link := s.clientBaseURL + page + "?token=" + url.QueryEscape(raw)
	body := "Zdravo " + user.FirstName + ",\n\n" + intro + "\n" + link +
		"\n\nLink važi " + ttl.String() + " i može se iskoristiti samo jednom. " +
		"Ako niste vi tražili ovo, ignorišite poruku.\n"
	if err := s.mailer.Send(user.Email, subject, body); err != nil {
		s.logger.Error("recovery: sending mail failed", "error", err)
		return
	}
	s.logger.Info("recovery link sent", "type", typ, "userId", user.ID.Hex())
}

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	email, ok := s.readEmail(w, r, "password_forgot")
	if !ok {
		return
	}
	go s.sendRecoveryLink(email, TokenReset, s.resetTTL, "/reset.html",
		"Reset lozinke", "Za postavljanje nove lozinke kliknite na link:")
	platform.WriteJSON(w, http.StatusOK, map[string]string{"message": genericLinkMessage})
}

type resetRequest struct {
	Token          string `json:"token"`
	Password       string `json:"password"`
	PasswordRepeat string `json:"passwordRepeat"`
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetRequest
	if err := decodeJSON(w, r, &req); err != nil || req.Token == "" || len(req.Token) > 100 {
		s.logger.Warn("validation failed", "endpoint", "password_reset", "reason", "bad_request")
		platform.WriteError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	if fields := checkNewPassword(req.Password, req.PasswordRepeat, "password", "passwordRepeat"); len(fields) > 0 {
		s.writeFieldErrors(w, "password_reset", fields)
		return
	}

	ctx := r.Context()
	tokenHash := hashToken(req.Token)
	userID, err := s.store.PeekToken(ctx, TokenReset, tokenHash)
	if errors.Is(err, ErrInvalidToken) {
		s.logger.Warn("invalid reset token used")
		platform.WriteError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	if err != nil {
		s.internalError(w, "peek reset token failed", err)
		return
	}
	user, err := s.store.FindUserByID(ctx, userID)
	if err != nil {
		s.internalError(w, "find user failed", err)
		return
	}
	if sameAsCurrent(user, req.Password) {
		s.writeFieldErrors(w, "password_reset", map[string]string{
			"password": "Nova lozinka mora biti drugačija od trenutne"})
		return
	}

	if _, err := s.store.ConsumeToken(ctx, TokenReset, tokenHash); err != nil {
		s.logger.Warn("invalid reset token used")
		platform.WriteError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err == nil {
		err = s.store.SetPassword(ctx, user.ID, string(hash))
	}
	if err != nil {
		s.internalError(w, "set password failed", err)
		return
	}
	s.logger.Info("password reset", "userId", user.ID.Hex())
	platform.WriteJSON(w, http.StatusOK, map[string]string{"message": "password changed"})
}

type changeRequest struct {
	CurrentPassword   string `json:"currentPassword"`
	NewPassword       string `json:"newPassword"`
	NewPasswordRepeat string `json:"newPasswordRepeat"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	var req changeRequest
	if err := decodeJSON(w, r, &req); err != nil ||
		req.CurrentPassword == "" || len(req.CurrentPassword) > 72 {
		s.logger.Warn("validation failed", "endpoint", "password_change", "reason", "bad_request")
		platform.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if fields := checkNewPassword(req.NewPassword, req.NewPasswordRepeat,
		"newPassword", "newPasswordRepeat"); len(fields) > 0 {
		s.writeFieldErrors(w, "password_change", fields)
		return
	}

	userID, err := bson.ObjectIDFromHex(claims.Subject)
	if err != nil {
		s.logger.Warn("access denied", "reason", "bad_subject")
		platform.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}
	ctx := r.Context()
	user, err := s.store.FindUserByID(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		s.logger.Warn("access denied", "reason", "user_not_found")
		platform.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}
	if err != nil {
		s.internalError(w, "find user failed", err)
		return
	}

	if time.Now().Before(user.LockedUntil) {
		writeCoded(w, http.StatusTooManyRequests, "too many failed attempts, try again later", "account_locked")
		return
	}
	if !sameAsCurrent(user, req.CurrentPassword) {
		locked, err := s.store.RecordFailedLogin(ctx, user.ID, s.maxFailures, s.lockoutFor)
		if err != nil {
			s.logger.Error("record failed login failed", "error", err)
		}
		s.logger.Warn("password change denied", "reason", "wrong_current_password",
			"userId", user.ID.Hex(), "locked", locked)
		writeCoded(w, http.StatusForbidden, "current password is incorrect", "wrong_password")
		return
	}
	if tooNew(user.PasswordChangedAt, s.passwordMinAge) {
		s.logger.Warn("password change denied", "reason", "password_too_new", "userId", user.ID.Hex())
		writeCoded(w, http.StatusForbidden,
			"password was changed too recently, try again later", "password_too_new")
		return
	}
	if sameAsCurrent(user, req.NewPassword) {
		s.writeFieldErrors(w, "password_change", map[string]string{
			"newPassword": "Nova lozinka mora biti drugačija od trenutne"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcryptCost)
	if err == nil {
		err = s.store.SetPassword(ctx, user.ID, string(hash))
	}
	if err != nil {
		s.internalError(w, "set password failed", err)
		return
	}
	s.logger.Info("password changed", "userId", user.ID.Hex())
	platform.WriteJSON(w, http.StatusOK, map[string]string{"message": "password changed"})
}

func (s *Server) handleMagicRequest(w http.ResponseWriter, r *http.Request) {
	email, ok := s.readEmail(w, r, "magic_request")
	if !ok {
		return
	}
	go s.sendRecoveryLink(email, TokenMagic, s.magicTTL, "/magic.html",
		"Prijava magičnim linkom", "Za prijavu bez lozinke kliknite na link:")
	platform.WriteJSON(w, http.StatusOK, map[string]string{"message": genericLinkMessage})
}

type tokenRequest struct {
	Token string `json:"token"`
}

func (s *Server) handleMagicLogin(w http.ResponseWriter, r *http.Request) {
	var req tokenRequest
	if err := decodeJSON(w, r, &req); err != nil || req.Token == "" || len(req.Token) > 100 {
		s.logger.Warn("validation failed", "endpoint", "magic_login", "reason", "bad_request")
		platform.WriteError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	ctx := r.Context()
	userID, err := s.store.ConsumeToken(ctx, TokenMagic, hashToken(req.Token))
	if errors.Is(err, ErrInvalidToken) {
		s.logger.Warn("login attempt", "success", false, "method", "magic_link", "reason", "invalid_token")
		platform.WriteError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	if err != nil {
		s.internalError(w, "consume magic token failed", err)
		return
	}
	user, err := s.store.FindUserByID(ctx, userID)
	if err != nil {
		s.internalError(w, "find user failed", err)
		return
	}
	if user.Status != StatusActive {
		s.logger.Warn("login attempt", "success", false, "method", "magic_link", "reason", "not_active", "userId", user.ID.Hex())
		writeCoded(w, http.StatusForbidden, "account not active", "account_not_confirmed")
		return
	}
	if s.passwordExpired(user) {
		s.logger.Warn("login attempt", "success", false, "method", "magic_link", "reason", "password_expired", "userId", user.ID.Hex())
		writeCoded(w, http.StatusForbidden, "password expired, reset it by email", "password_expired")
		return
	}
	s.writeSession(w, user, "magic_link")
}
