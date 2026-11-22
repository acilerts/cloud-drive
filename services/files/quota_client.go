package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type QuotaManager interface {
	Reserve(ctx context.Context, userID string, bytes int64) (string, error)
	Commit(ctx context.Context, userID string, reservationID string) error
	Release(ctx context.Context, userID string, reservationID string) error
	GetReservationStatus(ctx context.Context, userID string, reservationID string) (string, error)
}

type QuotaClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewQuotaClient(baseURL string, token string) *QuotaClient {
	return &QuotaClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type reserveQuotaRequest struct {
	Bytes int64 `json:"bytes"`
}

type reserveQuotaResponse struct {
	ReservationID string `json:"reservationId"`
	Status        string `json:"status"`
}

func (client *QuotaClient) Reserve(
	ctx context.Context,
	userID string,
	size int64,
) (string, error) {
	if userID == "" || size <= 0 {
		return "", fmt.Errorf("invalid quota reservation parameters")
	}

	body, err := json.Marshal(reserveQuotaRequest{
		Bytes: size,
	})
	if err != nil {
		return "", fmt.Errorf("encode quota reservation: %w", err)
	}

	path := fmt.Sprintf(
		"/internal/users/%s/quota/reservations",
		url.PathEscape(userID),
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		client.baseURL+path,
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("create quota reservation request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", client.token)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request quota reservation: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", readQuotaError(resp)
	}

	var result reserveQuotaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode quota reservation response: %w", err)
	}

	if result.ReservationID == "" {
		return "", fmt.Errorf("quota service returned an empty reservation ID")
	}

	if result.Status != "reserved" {
		return "", fmt.Errorf("unexpected quota reservation status: %s", result.Status)
	}

	return result.ReservationID, nil
}

func (client *QuotaClient) Commit(
	ctx context.Context,
	userID string,
	reservationID string,
) error {
	return client.reservationAction(
		ctx,
		userID,
		reservationID,
		"commit",
	)
}

func (client *QuotaClient) Release(
	ctx context.Context,
	userID string,
	reservationID string,
) error {
	return client.reservationAction(
		ctx,
		userID,
		reservationID,
		"release",
	)
}

func (client *QuotaClient) GetReservationStatus(
	ctx context.Context,
	userID string,
	reservationID string,
) (string, error) {
	if userID == "" || reservationID == "" {
		return "", fmt.Errorf("invalid quota reservation parameters")
	}

	path := fmt.Sprintf(
		"/internal/users/%s/quota/reservations/%s",
		url.PathEscape(userID),
		url.PathEscape(reservationID),
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		client.baseURL+path,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("create quota status request: %w", err)
	}

	req.Header.Set("X-Internal-Token", client.token)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request quota status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", readQuotaError(resp)
	}

	var result struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode quota status response: %w", err)
	}

	switch result.Status {
	case "reserved", "committed", "released":
		return result.Status, nil
	default:
		return "", fmt.Errorf("unexpected quota reservation status: %s", result.Status)
	}
}

func (client *QuotaClient) reservationAction(
	ctx context.Context,
	userID string,
	reservationID string,
	action string,
) error {
	if userID == "" || reservationID == "" {
		return fmt.Errorf("invalid quota reservation parameters")
	}

	path := fmt.Sprintf(
		"/internal/users/%s/quota/reservations/%s/%s",
		url.PathEscape(userID),
		url.PathEscape(reservationID),
		action,
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		client.baseURL+path,
		nil,
	)
	if err != nil {
		return fmt.Errorf("create quota %s request: %w", action, err)
	}

	req.Header.Set("X-Internal-Token", client.token)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request quota %s: %w", action, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readQuotaError(resp)
	}

	return nil
}

func readQuotaError(resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Errorf("quota service returned HTTP %d", resp.StatusCode)
	}

	return fmt.Errorf(
		"quota service returned HTTP %d: %s",
		resp.StatusCode,
		strings.TrimSpace(string(body)),
	)
}
