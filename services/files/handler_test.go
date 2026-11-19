package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/acilerts/cloud-drive/internal/platform"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func createTestHandler(t *testing.T) (*Handler, string) {
	t.Helper()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}

	userID := bson.NewObjectID()

	token, err := platform.SignToken(
		privateKey,
		userID.Hex(),
		"user",
		time.Hour,
	)
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}

	return NewHandler(nil, publicKey), token
}

func TestCreateFolderWithoutAuthorization(t *testing.T) {
	handler := NewHandler(nil, nil)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/folders",
		strings.NewReader(`{"name":"Dokumenti"}`),
	)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d",
			http.StatusUnauthorized, response.Code)
	}
}

func TestCreateFolderWithInvalidAuthorizationScheme(t *testing.T) {
	handler := NewHandler(nil, nil)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/folders",
		strings.NewReader(`{"name":"Dokumenti"}`),
	)

	request.Header.Set("Authorization", "Basic some-token")

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d",
			http.StatusUnauthorized, response.Code)
	}
}

func TestCreateFolderWithInvalidJSON(t *testing.T) {
	handler, token := createTestHandler(t)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/folders",
		strings.NewReader(`{"name":`),
	)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d",
			http.StatusBadRequest, response.Code)
	}
}

func TestCreateFolderWithInvalidParentID(t *testing.T) {
	handler, token := createTestHandler(t)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/folders",
		strings.NewReader(`{"name":"Dokumenti","parentId":"nije-validan-id"}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d",
			http.StatusBadRequest, response.Code)
	}
}

func TestCreateFolderWithUnknownOwnerIDField(t *testing.T) {
	handler, token := createTestHandler(t)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/folders",
		strings.NewReader(`{"name":"Dokumenti","ownerId":"123456789012345678901234"}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d",
			http.StatusBadRequest, response.Code)
	}
}
