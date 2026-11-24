package main

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"

	"github.com/acilerts/cloud-drive/internal/platform"
)

const bcryptCost = 12

type Server struct {
	store         *Store
	mailer        *Mailer
	logger        *slog.Logger
	clientBaseURL string
	defaultQuota  int64
	confirmTTL    time.Duration

	jwtKey         ed25519.PrivateKey
	jwtPub         ed25519.PublicKey
	jwtTTL         time.Duration
	otpTTL         time.Duration
	passwordMaxAge time.Duration
	passwordMinAge time.Duration
	resetTTL       time.Duration
	magicTTL       time.Duration
	maxFailures    int
	lockoutFor     time.Duration
	dummyHash      []byte

	internalToken string
	quotaMax      int64
}

func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /register", s.handleRegister)
	mux.HandleFunc("POST /confirm", s.handleConfirm)
	mux.HandleFunc("POST /login", s.handleLogin)
	mux.HandleFunc("POST /login/verify", s.handleLoginVerify)
	mux.HandleFunc("POST /password/change", s.handleChangePassword)
	mux.HandleFunc("POST /password/forgot", s.handleForgotPassword)
	mux.HandleFunc("POST /password/reset", s.handleResetPassword)
	mux.HandleFunc("POST /magic/request", s.handleMagicRequest)
	mux.HandleFunc("POST /magic/login", s.handleMagicLogin)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func (s *Server) internalError(w http.ResponseWriter, msg string, err error) {
	s.logger.Error(msg, "error", err)
	platform.WriteError(w, http.StatusInternalServerError, "internal error")
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.logger.Warn("validation failed", "endpoint", "register", "reason", "invalid_json")
		platform.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Normalize()
	if fields := req.Validate(); len(fields) > 0 {
		names := make([]string, 0, len(fields))
		for k := range fields {
			names = append(names, k)
		}
		s.logger.Warn("validation failed", "endpoint", "register", "fields", names)
		platform.WriteJSON(w, http.StatusBadRequest,
			map[string]any{"error": "validation failed", "fields": fields})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		s.internalError(w, "bcrypt failed", err)
		return
	}

	now := time.Now().UTC()
	user := &User{
		FirstName:         req.FirstName,
		LastName:          req.LastName,
		Email:             req.Email,
		PasswordHash:      string(hash),
		Role:              RoleUser,
		Status:            StatusPending,
		QuotaBytes:        s.defaultQuota,
		PasswordChangedAt: now,
		CreatedAt:         now,
	}
	if err := s.store.CreateUser(r.Context(), user); err != nil {
		if errors.Is(err, ErrEmailTaken) {
			platform.WriteError(w, http.StatusConflict, "email already registered")
			return
		}
		s.internalError(w, "create user failed", err)
		return
	}

	raw, tokenHash, err := newToken()
	if err == nil {
		err = s.store.CreateToken(r.Context(), user.ID, TokenConfirm, tokenHash, s.confirmTTL)
	}
	if err != nil {
		_ = s.store.DeleteUser(r.Context(), user.ID)
		s.internalError(w, "create confirm token failed", err)
		return
	}

	link := s.clientBaseURL + "/confirm.html?token=" + url.QueryEscape(raw)
	body := "Zdravo " + user.FirstName + ",\n\nPotvrdite registraciju klikom na link:\n" + link +
		"\n\nLink važi ograničeno vreme i može se iskoristiti samo jednom.\n"
	if err := s.mailer.Send(user.Email, "Potvrda registracije", body); err != nil {
		_ = s.store.DeleteUser(r.Context(), user.ID)
		s.logger.Error("sending confirmation mail failed", "error", err)
		platform.WriteError(w, http.StatusServiceUnavailable, "could not send confirmation email")
		return
	}

	s.logger.Info("user registered", "userId", user.ID.Hex())
	platform.WriteJSON(w, http.StatusCreated,
		map[string]string{"message": "registration successful, check your email to confirm"})
}

type confirmRequest struct {
	Token string `json:"token"`
}

func (s *Server) handleConfirm(w http.ResponseWriter, r *http.Request) {
	var req confirmRequest
	if err := decodeJSON(w, r, &req); err != nil || req.Token == "" || len(req.Token) > 100 {
		s.logger.Warn("validation failed", "endpoint", "confirm", "reason", "bad_token_format")
		platform.WriteError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	userID, err := s.store.ConsumeToken(r.Context(), TokenConfirm, hashToken(req.Token))
	if err == nil {
		err = s.store.ActivateUser(r.Context(), userID)
	}
	if errors.Is(err, ErrInvalidToken) {
		s.logger.Warn("invalid confirm token used")
		platform.WriteError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	if err != nil {
		s.internalError(w, "confirm failed", err)
		return
	}
	s.logger.Info("account confirmed", "userId", userID.Hex())
	platform.WriteJSON(w, http.StatusOK, map[string]string{"message": "account confirmed"})
}

type reserveQuotaRequest struct {
	Bytes int64 `json:"bytes"`
}

func parseQuotaReservationIDs(r *http.Request) (bson.ObjectID, bson.ObjectID, error) {
	userID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		return bson.NilObjectID, bson.NilObjectID, err
	}

	reservationID, err := bson.ObjectIDFromHex(r.PathValue("reservationId"))
	if err != nil {
		return bson.NilObjectID, bson.NilObjectID, err
	}

	return userID, reservationID, nil
}

func (s *Server) handleQuotaReservationError(w http.ResponseWriter, err error, operation string) {
	switch {
	case errors.Is(err, ErrQuotaExceeded):
		platform.WriteError(w, http.StatusConflict, "quota exceeded")

	case errors.Is(err, ErrReservationNotFound), errors.Is(err, ErrNotFound):
		platform.WriteError(w, http.StatusNotFound, "resource not found")

	case errors.Is(err, ErrReservationInactive):
		platform.WriteError(w, http.StatusConflict, "reservation is not active")

	default:
		s.internalError(w, operation, err)
	}
}

func (s *Server) handleInternalReserveQuota(w http.ResponseWriter, r *http.Request) {
	userID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		platform.WriteError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req reserveQuotaRequest
	if err := decodeJSON(w, r, &req); err != nil || req.Bytes <= 0 {
		platform.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	reservation, err := s.store.ReserveQuota(r.Context(), userID, req.Bytes)
	if err != nil {
		s.handleQuotaReservationError(w, err, "reserve quota failed")
		return
	}

	platform.WriteJSON(w, http.StatusCreated, map[string]string{
		"reservationId": reservation.ID.Hex(),
		"status":        reservation.Status,
	})
}

func (s *Server) handleInternalCommitQuota(w http.ResponseWriter, r *http.Request) {
	userID, reservationID, err := parseQuotaReservationIDs(r)
	if err != nil {
		platform.WriteError(w, http.StatusBadRequest, "invalid user or reservation id")
		return
	}

	if err := s.store.CommitQuota(r.Context(), userID, reservationID); err != nil {
		s.handleQuotaReservationError(w, err, "commit quota failed")
		return
	}

	platform.WriteJSON(w, http.StatusOK, map[string]string{
		"status": ReservationCommitted,
	})
}

func (s *Server) handleInternalReleaseQuota(w http.ResponseWriter, r *http.Request) {
	userID, reservationID, err := parseQuotaReservationIDs(r)
	if err != nil {
		platform.WriteError(w, http.StatusBadRequest, "invalid user or reservation id")
		return
	}

	if err := s.store.ReleaseQuota(r.Context(), userID, reservationID); err != nil {
		s.handleQuotaReservationError(w, err, "release quota failed")
		return
	}

	platform.WriteJSON(w, http.StatusOK, map[string]string{
		"status": ReservationReleased,
	})
}

func (s *Server) handleInternalQuotaReservationStatus(w http.ResponseWriter, r *http.Request) {
	userID, reservationID, err := parseQuotaReservationIDs(r)
	if err != nil {
		platform.WriteError(w, http.StatusBadRequest, "invalid user or reservation id")
		return
	}

	status, err := s.store.GetQuotaReservationStatus(r.Context(), userID, reservationID)
	if err != nil {
		s.handleQuotaReservationError(w, err, "get quota reservation status failed")
		return
	}

	platform.WriteJSON(w, http.StatusOK, map[string]string{
		"status": status,
	})
}
