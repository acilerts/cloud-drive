package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/acilerts/cloud-drive/internal/platform"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	QuotaPending  = "pending"
	QuotaApproved = "approved"
	QuotaRejected = "rejected"
)

type QuotaRequest struct {
	ID             bson.ObjectID `bson:"_id"`
	UserID         bson.ObjectID `bson:"userId"`
	RequestedBytes int64         `bson:"requestedBytes"`
	Status         string        `bson:"status"`
	CreatedAt      time.Time     `bson:"createdAt"`
	DecidedAt      time.Time     `bson:"decidedAt"`
	DecidedBy      bson.ObjectID `bson:"decidedBy"`
}

type quotaRequestDTO struct {
	ID                string     `json:"id"`
	UserID            string     `json:"userId"`
	Email             string     `json:"email"`
	RequestedBytes    int64      `json:"requestedBytes"`
	CurrentQuotaBytes *int64     `json:"currentQuotaBytes,omitempty"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"createdAt"`
	DecidedAt         *time.Time `json:"decidedAt,omitempty"`
}

func toDTO(qr QuotaRequest) quotaRequestDTO {
	dto := quotaRequestDTO{
		ID:             qr.ID.Hex(),
		UserID:         qr.UserID.Hex(),
		RequestedBytes: qr.RequestedBytes,
		Status:         qr.Status,
		CreatedAt:      qr.CreatedAt,
	}

	if !qr.DecidedAt.IsZero() {
		t := qr.DecidedAt
		dto.DecidedAt = &t
	}
	return dto
}

func validateRequestedQuota(requested, current, max int64) string {
	if requested <= 0 {
		return "Kvota mora biti pozitivan broj bajtova"
	}
	if requested <= current {
		return "Trazena kvota mora biti veca od trenutne"
	}
	if requested > max {
		return "Trazena kvota prelazi dozvoljeni maksimum"
	}
	return ""
}

func (s *Server) QuotaRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /quota/requests", s.handleCreateQuotaRequest)
	mux.HandleFunc("GET /quota/requests/mine", s.handleMyQuotaRequests)
	mux.HandleFunc("GET /admin/users", s.handleAdminUsers)
	mux.HandleFunc("GET /admin/quota-requests", s.handleAdminQuotaRequests)
	mux.HandleFunc("POST /admin/quota-requests/{id}/approve", s.decideQuota(true))
	mux.HandleFunc("POST /admin/quota-requests/{id}/reject", s.decideQuota(false))
	mux.HandleFunc("GET /internal/users/{id}/quota", platform.RequireInternal(s.internalToken, s.logger, s.handleInternalQuota))
}

func (s *Server) authorize(w http.ResponseWriter, r *http.Request, role string) (*platform.Claims, bool) {
	claims, ok := s.authenticate(w, r)
	if !ok {
		return nil, false
	}
	if role != "" && claims.Role != role {
		s.logger.Warn("access denied", "reason", "insufficient_role", "path", r.URL.Path, "userId", claims.Subject, "role", claims.Role)
		platform.WriteError(w, http.StatusForbidden, "forbidden")
		return nil, false
	}
	return claims, true
}

type quotaRequestBody struct {
	RequestedBytes int64 `json:"requestedBytes"`
}

func (s *Server) handleCreateQuotaRequest(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.authorize(w, r, "")
	if !ok {
		return
	}
	var body quotaRequestBody
	if err := decodeJSON(w, r, &body); err != nil {
		s.logger.Warn("validation failed", "endpoint", "quota_request", "reason", "invalid_json")
		platform.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	userID, err := bson.ObjectIDFromHex(claims.Subject)
	if err != nil {
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
	if user.Status != StatusActive {
		platform.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if m := validateRequestedQuota(body.RequestedBytes, user.QuotaBytes, s.quotaMax); m != "" {
		s.writeFieldErrors(w, "quota_request", map[string]string{"requestedBytes": m})
		return
	}

	qr := &QuotaRequest{
		UserID:         user.ID,
		RequestedBytes: body.RequestedBytes,
		Status:         QuotaPending,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.store.CreateQuotaRequest(ctx, qr); err != nil {
		if errors.Is(err, ErrPendingExists) {
			writeCoded(w, http.StatusConflict, "a pending request already exists", "request_pending")
			return
		}
		s.internalError(w, "create quota request failed", err)
		return
	}
	s.logger.Info("quota request created", "userId", user.ID.Hex(),
		"requestId", qr.ID.Hex(), "requestedBytes", qr.RequestedBytes)
	platform.WriteJSON(w, http.StatusCreated, toDTO(*qr))
}

func (s *Server) handleMyQuotaRequests(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.authorize(w, r, "")
	if !ok {
		return
	}
	userID, err := bson.ObjectIDFromHex(claims.Subject)
	if err != nil {
		platform.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}
	ctx := r.Context()
	user, err := s.store.FindUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			platform.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		s.internalError(w, "find user failed", err)
		return
	}
	reqs, err := s.store.ListQuotaRequestsByUser(ctx, userID)
	if err != nil {
		s.internalError(w, "list quota requests failed", err)
		return
	}
	out := make([]quotaRequestDTO, 0, len(reqs))
	for _, qr := range reqs {
		out = append(out, toDTO(qr))
	}
	platform.WriteJSON(w, http.StatusOK, map[string]any{
		"quotaBytes": user.QuotaBytes,
		"requests":   out,
	})
}

type adminUserDTO struct {
	ID         string `json:"id"`
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	Status     string `json:"status"`
	QuotaBytes int64  `json:"quotaBytes"`
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.authorize(w, r, RoleAdmin)
	if !ok {
		return
	}
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		s.internalError(w, "list users failed", err)
		return
	}
	out := make([]adminUserDTO, 0, len(users))
	for _, u := range users {
		out = append(out, adminUserDTO{
			ID: u.ID.Hex(), FirstName: u.FirstName, LastName: u.LastName, Email: u.Email, Role: u.Role, Status: u.Status, QuotaBytes: u.QuotaBytes,
		})
	}

	s.logger.Info("admin action", "action", "list_users", "adminId", claims.Subject)
	platform.WriteJSON(w, http.StatusOK, map[string]any{"users": out})
}

func (s *Server) handleAdminQuotaRequests(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.authorize(w, r, RoleAdmin)
	if !ok {
		return
	}
	status := r.URL.Query().Get("status")
	if status == "" {
		status = QuotaPending
	}
	if status != QuotaPending && status != QuotaApproved && status != QuotaRejected {
		s.logger.Warn("validation failed", "endpoint", "admin_quota_requests", "reason", "bad_status")
		platform.WriteError(w, http.StatusBadRequest, "invalid status")
		return
	}

	ctx := r.Context()
	reqs, err := s.store.ListQuotaRequestsByStatus(ctx, status)
	if err != nil {
		s.internalError(w, "list quota requests failed", err)
		return
	}
	ids := make([]bson.ObjectID, 0, len(reqs))
	for _, qr := range reqs {
		ids = append(ids, qr.UserID)
	}
	users, err := s.store.FindUsersByIDs(ctx, ids)
	if err != nil {
		s.internalError(w, "find users failed", err)
		return
	}
	out := make([]quotaRequestDTO, 0, len(reqs))
	for _, qr := range reqs {
		dto := toDTO(qr)
		if u, found := users[qr.UserID]; found {
			dto.Email = u.Email
			q := u.QuotaBytes
			dto.CurrentQuotaBytes = &q
		}
		out = append(out, dto)
	}
	s.logger.Info("admin action", "action", "list_quota_requests", "adminId", claims.Subject, "status", status)
	platform.WriteJSON(w, http.StatusOK, map[string]any{"requests": out})
}

func (s *Server) decideQuota(approve bool) http.HandlerFunc {
	action := "quota_request_reject"
	status := QuotaRejected
	if approve {
		action = "quota_request_approve"
		status = QuotaApproved
	}
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := s.authorize(w, r, RoleAdmin)
		if !ok {
			return
		}
		id, err := bson.ObjectIDFromHex(r.PathValue("id"))
		if err != nil {
			s.logger.Warn("validation failed", "endpoint", action, "reason", "bad_id")
			platform.WriteError(w, http.StatusBadRequest, "invalid id")
			return
		}
		adminID, err := bson.ObjectIDFromHex(claims.Subject)
		if err != nil {
			platform.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
		defer cancel()

		qr, err := s.store.DecideQuotaRequest(ctx, id, adminID, status)
		if errors.Is(err, ErrNotFound) {
			platform.WriteError(w, http.StatusNotFound, "request not found or already decided")
			return
		}
		if err != nil {
			s.internalError(w, "decide quota request failed", err)
			return
		}
		if approve {
			if err := s.store.SetQuota(ctx, qr.UserID, qr.RequestedBytes); err != nil {
				if rerr := s.store.ReopenQuotaRequest(ctx, qr.ID); rerr != nil {
					s.logger.Error("reopen quota request failed", "error", rerr, "requestId", qr.ID.Hex())
				}
				s.internalError(w, "set quota failed", err)
				return
			}
		}
		s.logger.Info("admin action", "action", action, "adminId", claims.Subject,
			"requestId", qr.ID.Hex(), "targetUserId", qr.UserID.Hex(), "requestedBytes", qr.RequestedBytes)
		platform.WriteJSON(w, http.StatusOK, toDTO(*qr))
	}
}

func (s *Server) handleInternalQuota(w http.ResponseWriter, r *http.Request) {
	id, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		platform.WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	user, err := s.store.FindUserByID(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		platform.WriteError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		s.internalError(w, "find user failed", err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, map[string]any{
		"userId":     user.ID.Hex(),
		"quotaBytes": user.QuotaBytes,
		"status":     user.Status,
	})
}
