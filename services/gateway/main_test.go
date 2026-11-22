package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func newTestGateway(t *testing.T, rps, burst, authRPM int) (http.Handler, *string) {
	t.Helper()

	var lastBackend string
	usersSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastBackend = "users:" + r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(usersSrv.Close)

	filesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastBackend = "files:" + r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(filesSrv.Close)

	notifSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastBackend = "notifications:" + r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(notifSrv.Close)

	clientDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(clientDir, "login.html"), []byte("<h1>Login</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}

	uURL, _ := url.Parse(usersSrv.URL)
	fURL, _ := url.Parse(filesSrv.URL)
	nURL, _ := url.Parse(notifSrv.URL)

	cfg := &Config{
		Port:             "8080",
		UsersURL:         uURL,
		FilesURL:         fURL,
		NotificationsURL: nURL,
		ClientDir:        clientDir,
		RateLimitRPS:     rps,
		RateLimitBurst:   burst,
		AuthRateLimitRPM: authRPM,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return newGatewayHandler(cfg, logger), &lastBackend
}

func TestGatewayRouting(t *testing.T) {
	h, lastBackend := newTestGateway(t, 100, 100, 100)

	cases := []struct {
		method      string
		path        string
		wantStatus  int
		wantBackend string
	}{
		{"GET", "/health", http.StatusOK, ""},
		{"GET", "/", http.StatusFound, ""},
		{"GET", "/login.html", http.StatusOK, ""},
		{"POST", "/register", http.StatusOK, "users:/register"},
		{"POST", "/confirm", http.StatusOK, "users:/confirm"},
		{"POST", "/login", http.StatusOK, "users:/login"},
		{"POST", "/login/verify", http.StatusOK, "users:/login/verify"},
		{"POST", "/password/change", http.StatusOK, "users:/password/change"},
		{"POST", "/password/forgot", http.StatusOK, "users:/password/forgot"},
		{"POST", "/password/reset", http.StatusOK, "users:/password/reset"},
		{"POST", "/magic/request", http.StatusOK, "users:/magic/request"},
		{"POST", "/magic/login", http.StatusOK, "users:/magic/login"},
		{"POST", "/quota/requests", http.StatusOK, "users:/quota/requests"},
		{"GET", "/quota/requests/mine", http.StatusOK, "users:/quota/requests/mine"},
		{"GET", "/admin/users", http.StatusOK, "users:/admin/users"},
		{"GET", "/admin/quota-requests", http.StatusOK, "users:/admin/quota-requests"},
		{"POST", "/admin/quota-requests/abc/approve", http.StatusOK, "users:/admin/quota-requests/abc/approve"},
		{"GET", "/folders", http.StatusOK, "files:/folders"},
		{"POST", "/files", http.StatusOK, "files:/files"},
		{"GET", "/notifications", http.StatusOK, "notifications:/notifications"},
		{"GET", "/internal/users/123/quota", http.StatusNotFound, ""},
		{"GET", "/unknown-route", http.StatusNotFound, ""},
	}

	for _, tc := range cases {
		*lastBackend = ""
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != tc.wantStatus {
			t.Errorf("%s %s: status = %d, want %d", tc.method, tc.path, rec.Code, tc.wantStatus)
		}
		if *lastBackend != tc.wantBackend {
			t.Errorf("%s %s: backend = %q, want %q", tc.method, tc.path, *lastBackend, tc.wantBackend)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s %s: missing X-Content-Type-Options header", tc.method, tc.path)
		}
	}
}

func TestGatewayRateLimit(t *testing.T) {
	h, _ := newTestGateway(t, 1, 2, 2)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "10.0.0.5:5555"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "10.0.0.5:5555"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd request: status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on 429 response")
	}
}

func TestBuildSelfSignedTLS(t *testing.T) {
	cfg, err := buildTLSConfig("", "", true)
	if err != nil {
		t.Fatalf("buildTLSConfig failed: %v", err)
	}
	if cfg == nil || len(cfg.Certificates) != 1 {
		t.Fatal("expected generated self-signed certificate")
	}
}
