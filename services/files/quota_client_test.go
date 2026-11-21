package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQuotaClientReserveSuccessfully(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}

			if r.URL.Path != "/internal/users/user-123/quota/reservations" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}

			if r.Header.Get("X-Internal-Token") != "test-token" {
				t.Error("missing or invalid internal token")
			}

			var request reserveQuotaRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode request: %v", err)
			}

			if request.Bytes != 1024 {
				t.Errorf("expected 1024 bytes, got %d", request.Bytes)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)

			_ = json.NewEncoder(w).Encode(reserveQuotaResponse{
				ReservationID: "reservation-123",
				Status:        "reserved",
			})
		},
	))
	defer server.Close()

	client := NewQuotaClient(server.URL, "test-token")

	reservationID, err := client.Reserve(
		t.Context(),
		"user-123",
		1024,
	)
	if err != nil {
		t.Fatalf("Reserve returned error: %v", err)
	}

	if reservationID != "reservation-123" {
		t.Errorf("unexpected reservation ID: %s", reservationID)
	}
}

func TestQuotaClientReserveRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "quota exceeded", http.StatusForbidden)
		},
	))
	defer server.Close()

	client := NewQuotaClient(server.URL, "test-token")

	_, err := client.Reserve(
		t.Context(),
		"user-123",
		1024,
	)
	if err == nil {
		t.Fatal("expected quota reservation error")
	}
}

func TestQuotaClientCommitAndRelease(t *testing.T) {
	requests := 0

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			requests++

			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}

			if r.Header.Get("X-Internal-Token") != "test-token" {
				t.Error("missing or invalid internal token")
			}

			switch r.URL.Path {
			case "/internal/users/user-123/quota/reservations/reservation-123/commit":
			case "/internal/users/user-123/quota/reservations/reservation-123/release":
			default:
				t.Errorf("unexpected path: %s", r.URL.Path)
			}

			w.WriteHeader(http.StatusNoContent)
		},
	))
	defer server.Close()

	client := NewQuotaClient(server.URL, "test-token")

	if err := client.Commit(
		t.Context(),
		"user-123",
		"reservation-123",
	); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if err := client.Release(
		t.Context(),
		"user-123",
		"reservation-123",
	); err != nil {
		t.Fatalf("Release returned error: %v", err)
	}

	if requests != 2 {
		t.Errorf("expected 2 requests, got %d", requests)
	}
}

func TestQuotaClientGetReservationStatusSuccessfully(t *testing.T) {
	statuses := []string{"reserved", "committed", "released"}
	requests := 0

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			requests++

			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}

			if r.URL.Path != "/internal/users/user-123/quota/reservations/reservation-123" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}

			if r.Header.Get("X-Internal-Token") != "test-token" {
				t.Error("missing or invalid internal token")
			}

			status := statuses[requests-1]

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"reservationId": "reservation-123",
				"status":        status,
			})
		},
	))
	defer server.Close()

	client := NewQuotaClient(server.URL, "test-token")

	for _, expectedStatus := range statuses {
		status, err := client.GetReservationStatus(
			t.Context(),
			"user-123",
			"reservation-123",
		)
		if err != nil {
			t.Fatalf("GetReservationStatus returned error: %v", err)
		}

		if status != expectedStatus {
			t.Errorf("expected status %s, got %s", expectedStatus, status)
		}
	}
}

func TestQuotaClientGetReservationStatusRejectsUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "unknown",
			})
		},
	))
	defer server.Close()

	client := NewQuotaClient(server.URL, "test-token")

	_, err := client.GetReservationStatus(
		t.Context(),
		"user-123",
		"reservation-123",
	)

	if err == nil {
		t.Fatal("expected error for unexpected reservation status")
	}
}
