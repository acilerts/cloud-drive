package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/acilerts/cloud-drive/internal/platform"
)

func TestInternalReserveQuotaHandler(t *testing.T) {
	store := newQuotaTestStore(t)
	userID := createQuotaTestUser(t, store, 1000, 100, StatusActive)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := &Server{
		store:         store,
		logger:        logger,
		internalToken: "test-token",
	}

	mux := http.NewServeMux()
	server.QuotaRoutes(mux)

	body := []byte(`{"bytes":300}`)
	req := httptest.NewRequest(
		http.MethodPost,
		"/internal/users/"+userID.Hex()+"/quota/reservations",
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(platform.InternalTokenHeader, "test-token")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusCreated,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response struct {
		ReservationID bson.ObjectID `json:"reservationId"`
		Status        string        `json:"status"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.ReservationID.IsZero() {
		t.Fatal("expected a non-zero reservation ID")
	}

	if response.Status != ReservationReserved {
		t.Fatalf("expected status %q, got %q", ReservationReserved, response.Status)
	}
}

func TestInternalReserveQuotaHandlerUnauthorized(t *testing.T) {
	store := newQuotaTestStore(t)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := &Server{
		store:         store,
		logger:        logger,
		internalToken: "test-token",
	}

	mux := http.NewServeMux()
	server.QuotaRoutes(mux)

	req := httptest.NewRequest(
		http.MethodPost,
		"/internal/users/"+bson.NewObjectID().Hex()+"/quota/reservations",
		bytes.NewReader([]byte(`{"bytes":300}`)),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusUnauthorized,
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestInternalQuotaReservationLifecycle(t *testing.T) {
	store := newQuotaTestStore(t)
	userID := createQuotaTestUser(t, store, 1000, 100, StatusActive)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := &Server{
		store:         store,
		logger:        logger,
		internalToken: "test-token",
	}

	mux := http.NewServeMux()
	server.QuotaRoutes(mux)

	sendRequest := func(method, path string, body []byte) *httptest.ResponseRecorder {
		t.Helper()

		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(platform.InternalTokenHeader, "test-token")

		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, req)
		return recorder
	}

	reserveResponse := sendRequest(
		http.MethodPost,
		"/internal/users/"+userID.Hex()+"/quota/reservations",
		[]byte(`{"bytes":300}`),
	)

	if reserveResponse.Code != http.StatusCreated {
		t.Fatalf("reserve: expected %d, got %d: %s",
			http.StatusCreated, reserveResponse.Code, reserveResponse.Body.String())
	}

	var reservation struct {
		ReservationID bson.ObjectID `json:"reservationId"`
		Status        string        `json:"status"`
	}

	if err := json.NewDecoder(reserveResponse.Body).Decode(&reservation); err != nil {
		t.Fatalf("decode reservation: %v", err)
	}

	if reservation.ReservationID.IsZero() {
		t.Fatal("expected a valid reservation ID")
	}

	reservationPath := "/internal/users/" + userID.Hex() +
		"/quota/reservations/" + reservation.ReservationID.Hex()

	commitResponse := sendRequest(
		http.MethodPost,
		reservationPath+"/commit",
		nil,
	)

	if commitResponse.Code != http.StatusOK {
		t.Fatalf("commit: expected %d, got %d: %s",
			http.StatusOK, commitResponse.Code, commitResponse.Body.String())
	}

	statusResponse := sendRequest(
		http.MethodGet,
		reservationPath,
		nil,
	)

	if statusResponse.Code != http.StatusOK {
		t.Fatalf("status: expected %d, got %d: %s",
			http.StatusOK, statusResponse.Code, statusResponse.Body.String())
	}

	var status struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(statusResponse.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}

	if status.Status != ReservationCommitted {
		t.Fatalf("expected status %q, got %q",
			ReservationCommitted, status.Status)
	}

	user, err := store.FindUserByID(t.Context(), userID)
	if err != nil {
		t.Fatalf("find user: %v", err)
	}

	if user.UsedBytes != 400 {
		t.Fatalf("expected UsedBytes=400, got %d", user.UsedBytes)
	}

	if user.ReservedBytes != 0 {
		t.Fatalf("expected ReservedBytes=0, got %d", user.ReservedBytes)
	}
}

func TestInternalQuotaReservationRelease(t *testing.T) {
	store := newQuotaTestStore(t)
	userID := createQuotaTestUser(t, store, 1000, 100, StatusActive)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := &Server{
		store:         store,
		logger:        logger,
		internalToken: "test-token",
	}

	mux := http.NewServeMux()
	server.QuotaRoutes(mux)

	sendRequest := func(method, path string, body []byte) *httptest.ResponseRecorder {
		t.Helper()

		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(platform.InternalTokenHeader, "test-token")

		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, req)
		return recorder
	}

	reserveResponse := sendRequest(
		http.MethodPost,
		"/internal/users/"+userID.Hex()+"/quota/reservations",
		[]byte(`{"bytes":300}`),
	)

	if reserveResponse.Code != http.StatusCreated {
		t.Fatalf("reserve: expected %d, got %d: %s",
			http.StatusCreated, reserveResponse.Code, reserveResponse.Body.String())
	}

	var reservation struct {
		ReservationID bson.ObjectID `json:"reservationId"`
	}

	if err := json.NewDecoder(reserveResponse.Body).Decode(&reservation); err != nil {
		t.Fatalf("decode reservation: %v", err)
	}

	reservationPath := "/internal/users/" + userID.Hex() +
		"/quota/reservations/" + reservation.ReservationID.Hex()

	releaseResponse := sendRequest(
		http.MethodPost,
		reservationPath+"/release",
		nil,
	)

	if releaseResponse.Code != http.StatusOK {
		t.Fatalf("release: expected %d, got %d: %s",
			http.StatusOK, releaseResponse.Code, releaseResponse.Body.String())
	}

	statusResponse := sendRequest(
		http.MethodGet,
		reservationPath,
		nil,
	)

	if statusResponse.Code != http.StatusOK {
		t.Fatalf("status: expected %d, got %d: %s",
			http.StatusOK, statusResponse.Code, statusResponse.Body.String())
	}

	var status struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(statusResponse.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}

	if status.Status != ReservationReleased {
		t.Fatalf("expected status %q, got %q",
			ReservationReleased, status.Status)
	}

	user, err := store.FindUserByID(t.Context(), userID)
	if err != nil {
		t.Fatalf("find user: %v", err)
	}

	if user.UsedBytes != 100 {
		t.Fatalf("expected UsedBytes=100, got %d", user.UsedBytes)
	}

	if user.ReservedBytes != 0 {
		t.Fatalf("expected ReservedBytes=0, got %d", user.ReservedBytes)
	}
}

func TestInternalReserveQuotaHandlerInvalidUserID(t *testing.T) {
	store := newQuotaTestStore(t)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := &Server{
		store:         store,
		logger:        logger,
		internalToken: "test-token",
	}

	mux := http.NewServeMux()
	server.QuotaRoutes(mux)

	req := httptest.NewRequest(
		http.MethodPost,
		"/internal/users/invalid-id/quota/reservations",
		bytes.NewReader([]byte(`{"bytes":300}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(platform.InternalTokenHeader, "test-token")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusBadRequest,
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestInternalQuotaReservationStatusNotFound(t *testing.T) {
	store := newQuotaTestStore(t)
	userID := createQuotaTestUser(t, store, 1000, 100, StatusActive)
	reservationID := bson.NewObjectID()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := &Server{
		store:         store,
		logger:        logger,
		internalToken: "test-token",
	}

	mux := http.NewServeMux()
	server.QuotaRoutes(mux)

	path := "/internal/users/" + userID.Hex() +
		"/quota/reservations/" + reservationID.Hex()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set(platform.InternalTokenHeader, "test-token")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusNotFound,
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestInternalReserveQuotaHandlerQuotaExceeded(t *testing.T) {
	store := newQuotaTestStore(t)
	userID := createQuotaTestUser(t, store, 1000, 100, StatusActive)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := &Server{
		store:         store,
		logger:        logger,
		internalToken: "test-token",
	}

	mux := http.NewServeMux()
	server.QuotaRoutes(mux)

	req := httptest.NewRequest(
		http.MethodPost,
		"/internal/users/"+userID.Hex()+"/quota/reservations",
		bytes.NewReader([]byte(`{"bytes":950}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(platform.InternalTokenHeader, "test-token")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusConflict {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusConflict,
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestInternalReserveQuotaHandlerInvalidBytes(t *testing.T) {
	store := newQuotaTestStore(t)
	userID := createQuotaTestUser(t, store, 1000, 100, StatusActive)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := &Server{
		store:         store,
		logger:        logger,
		internalToken: "test-token",
	}

	mux := http.NewServeMux()
	server.QuotaRoutes(mux)

	req := httptest.NewRequest(
		http.MethodPost,
		"/internal/users/"+userID.Hex()+"/quota/reservations",
		bytes.NewReader([]byte(`{"bytes":0}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(platform.InternalTokenHeader, "test-token")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusBadRequest,
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestInternalReserveQuotaHandlerInvalidJSON(t *testing.T) {
	store := newQuotaTestStore(t)
	userID := createQuotaTestUser(t, store, 1000, 100, StatusActive)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := &Server{
		store:         store,
		logger:        logger,
		internalToken: "test-token",
	}

	mux := http.NewServeMux()
	server.QuotaRoutes(mux)

	req := httptest.NewRequest(
		http.MethodPost,
		"/internal/users/"+userID.Hex()+"/quota/reservations",
		bytes.NewReader([]byte(`{"bytes":`)),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(platform.InternalTokenHeader, "test-token")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusBadRequest,
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestInternalQuotaReservationStatusInvalidID(t *testing.T) {
	store := newQuotaTestStore(t)
	userID := createQuotaTestUser(t, store, 1000, 100, StatusActive)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := &Server{
		store:         store,
		logger:        logger,
		internalToken: "test-token",
	}

	mux := http.NewServeMux()
	server.QuotaRoutes(mux)

	path := "/internal/users/" + userID.Hex() +
		"/quota/reservations/invalid-id"

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set(platform.InternalTokenHeader, "test-token")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusBadRequest,
			recorder.Code,
			recorder.Body.String(),
		)
	}
}
