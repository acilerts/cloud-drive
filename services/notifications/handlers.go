package main

import (
	"context"
	"crypto/ed25519"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/acilerts/cloud-drive/internal/platform"
)

type Server struct {
	store         *Store
	logger        *slog.Logger
	jwtPub        ed25519.PublicKey
	internalToken string
}

func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /notifications", s.handleList)
	mux.HandleFunc("POST /internal/notifications",
		platform.RequireInternal(s.internalToken, s.logger, s.handleCreate))
}

type notificationDTO struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Title      string    `json:"title"`
	Message    string    `json:"message"`
	ResourceID string    `json:"resourceId,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

func toDTO(n Notification) notificationDTO {
	return notificationDTO{
		ID:         n.ID.String(),
		Type:       n.Kind,
		Title:      n.Title,
		Message:    n.Message,
		ResourceID: n.ResourceID,
		CreatedAt:  n.ID.Time().UTC(),
	}
}

func (s *Server) internalError(w http.ResponseWriter, msg string, err error) {
	s.logger.Error(msg, "error", err)
	platform.WriteError(w, http.StatusInternalServerError, "internal error")
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	claims, ok := platform.Authenticate(w, r, s.jwtPub, s.logger)
	if !ok {
		return
	}
	if !userIDPattern.MatchString(claims.Subject) {
		s.logger.Warn("access denied", "reason", "bad_subject")
		platform.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			s.logger.Warn("validation failed", "endpoint", "notifications_list", "reason", "bad_limit")
			platform.WriteError(w, http.StatusBadRequest, "limit must be between 1 and 100")
			return
		}
		limit = n
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	items, err := s.store.ListByUser(ctx, claims.Subject, limit)
	if err != nil {
		s.internalError(w, "list notifications failed", err)
		return
	}
	out := make([]notificationDTO, 0, len(items))
	for _, n := range items {
		out = append(out, toDTO(n))
	}
	platform.WriteJSON(w, http.StatusOK, map[string]any{"notifications": out})
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := platform.DecodeJSON(w, r, &req); err != nil {
		s.logger.Warn("validation failed", "endpoint", "notifications_create", "reason", "invalid_json")
		platform.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if fields := req.validate(); len(fields) > 0 {
		names := make([]string, 0, len(fields))
		for k := range fields {
			names = append(names, k)
		}
		s.logger.Warn("validation failed", "endpoint", "notifications_create", "fields", names)
		platform.WriteJSON(w, http.StatusBadRequest,
			map[string]any{"error": "validation failed", "fields": fields})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	n := &Notification{
		UserID: req.UserID, Kind: req.Kind, Title: req.Title,
		Message: req.Message, ResourceID: req.ResourceID,
	}
	if err := s.store.Create(ctx, n); err != nil {
		s.internalError(w, "create notification failed", err)
		return
	}
	s.logger.Info("notification created", "userId", n.UserID, "type", n.Kind)
	platform.WriteJSON(w, http.StatusCreated, toDTO(*n))
}
