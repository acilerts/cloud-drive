package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/acilerts/cloud-drive/internal/platform"
)

type Config struct {
	Port             string
	UsersURL         *url.URL
	FilesURL         *url.URL
	NotificationsURL *url.URL
	ClientDir        string

	TLSEnabled  bool
	TLSCertFile string
	TLSKeyFile  string

	RateLimitRPS     int
	RateLimitBurst   int
	AuthRateLimitRPM int
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("gateway service stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	port, err := platform.Required("GATEWAY_PORT")
	if err != nil {
		return err
	}
	usersRaw, err := platform.Required("USERS_SERVICE_URL")
	if err != nil {
		return err
	}
	filesRaw, err := platform.Required("FILES_SERVICE_URL")
	if err != nil {
		return err
	}
	notifRaw, err := platform.Required("NOTIFICATIONS_SERVICE_URL")
	if err != nil {
		return err
	}
	clientDir := platform.Default("CLIENT_DIR", "/client")

	tlsEnabled := strings.EqualFold(platform.Default("TLS_ENABLED", "false"), "true")
	tlsCertFile := platform.Default("TLS_CERT_FILE", "")
	tlsKeyFile := platform.Default("TLS_KEY_FILE", "")

	rateRPS, err := platform.Int("RATE_LIMIT_RPS", 20)
	if err != nil {
		return err
	}
	rateBurst, err := platform.Int("RATE_LIMIT_BURST", 40)
	if err != nil {
		return err
	}
	authRPM, err := platform.Int("AUTH_RATE_LIMIT_RPM", 20)
	if err != nil {
		return err
	}

	usersURL, err := parseServiceURL("USERS_SERVICE_URL", usersRaw)
	if err != nil {
		return err
	}
	filesURL, err := parseServiceURL("FILES_SERVICE_URL", filesRaw)
	if err != nil {
		return err
	}
	notifURL, err := parseServiceURL("NOTIFICATIONS_SERVICE_URL", notifRaw)
	if err != nil {
		return err
	}

	cfg := &Config{
		Port:             port,
		UsersURL:         usersURL,
		FilesURL:         filesURL,
		NotificationsURL: notifURL,
		ClientDir:        clientDir,
		TLSEnabled:       tlsEnabled,
		TLSCertFile:      tlsCertFile,
		TLSKeyFile:       tlsKeyFile,
		RateLimitRPS:     rateRPS,
		RateLimitBurst:   rateBurst,
		AuthRateLimitRPM: authRPM,
	}

	tlsCfg, err := buildTLSConfig(cfg.TLSCertFile, cfg.TLSKeyFile, cfg.TLSEnabled)
	if err != nil {
		return err
	}

	handler := newGatewayHandler(cfg, logger)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		TLSConfig:         tlsCfg,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", srv.Addr, err)
	}
	if tlsCfg != nil {
		ln = tls.NewListener(ln, tlsCfg)
	}

	serverErr := make(chan error, 1)
	go func() {
		scheme := "http"
		if tlsCfg != nil {
			scheme = "https"
		}
		logger.Info("api gateway listening", "port", cfg.Port, "scheme", scheme)
		serverErr <- srv.Serve(ln)
	}()

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		logger.Info("gateway shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}

	return nil
}

func parseServiceURL(key, raw string) (*url.URL, error) {
	serviceURL, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", key, err)
	}
	if serviceURL.Scheme == "" || serviceURL.Host == "" {
		return nil, fmt.Errorf("invalid %s: URL must contain scheme and host", key)
	}

	return serviceURL, nil
}

func isUsersPath(path string) bool {
	return path == "/me" ||
		path == "/register" ||
		path == "/confirm" ||
		path == "/login" || strings.HasPrefix(path, "/login/") ||
		strings.HasPrefix(path, "/password/") ||
		strings.HasPrefix(path, "/magic/") ||
		strings.HasPrefix(path, "/quota/") ||
		strings.HasPrefix(path, "/admin/") ||
		path == "/users" || strings.HasPrefix(path, "/users/")
}

func isFilesPath(path string) bool {
	return path == "/files" || strings.HasPrefix(path, "/files/") ||
		path == "/folders" || strings.HasPrefix(path, "/folders/")
}

func isNotificationsPath(path string) bool {
	return path == "/notifications" || strings.HasPrefix(path, "/notifications/")
}

func setSecurityHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self'")
	if r.TLS != nil {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
}

func newGatewayHandler(cfg *Config, logger *slog.Logger) http.Handler {
	usersProxy := httputil.NewSingleHostReverseProxy(cfg.UsersURL)
	filesProxy := httputil.NewSingleHostReverseProxy(cfg.FilesURL)
	notifProxy := httputil.NewSingleHostReverseProxy(cfg.NotificationsURL)

	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Warn("proxy error", "path", r.URL.Path, "error", err)
		platform.WriteError(w, http.StatusBadGateway, "target service is unavailable")
	}
	usersProxy.ErrorHandler = errorHandler
	filesProxy.ErrorHandler = errorHandler
	notifProxy.ErrorHandler = errorHandler

	var apiLimiter *rateLimiter
	if cfg.RateLimitRPS > 0 && cfg.RateLimitBurst > 0 {
		apiLimiter = newRateLimiter(float64(cfg.RateLimitRPS), cfg.RateLimitBurst)
	}
	var authLimiter *rateLimiter
	if cfg.AuthRateLimitRPM > 0 {
		authLimiter = newRateLimiter(float64(cfg.AuthRateLimitRPM)/60.0, cfg.AuthRateLimitRPM)
	}

	absClientDir, _ := filepath.Abs(cfg.ClientDir)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w, r)

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		path := r.URL.Path

		if path == "/health" {
			platform.WriteJSON(w, http.StatusOK, map[string]string{"service": "gateway", "status": "ok"})
			return
		}

		// Nikada ne izlagati interne servisne rute spolja
		if path == "/internal" || strings.HasPrefix(path, "/internal/") {
			platform.WriteError(w, http.StatusNotFound, "not found")
			return
		}

		// Pocetna stranica
		if path == "/" {
			http.Redirect(w, r, "/login.html", http.StatusFound)
			return
		}

		// Serviranje klijentskih statickih fajlova ako fajl postoji unutar ClientDir
		// Koristimo ServeContent umesto FileServer/ServeFile kako /index.html ne bi radio 301 redirect na /
		cleanRel := filepath.Clean(strings.TrimPrefix(path, "/"))
		candidate := filepath.Join(absClientDir, cleanRel)
		if rel, err := filepath.Rel(absClientDir, candidate); err == nil &&
			rel != "." && !strings.HasPrefix(rel, "..") {
			if f, err := os.Open(candidate); err == nil {
				if info, statErr := f.Stat(); statErr == nil && !info.IsDir() {
					http.ServeContent(w, r, info.Name(), info.ModTime(), f)
					_ = f.Close()
					return
				}
				_ = f.Close()
			}
		}

		// Rate limiting za API rute (DoS zastita - zahtev 2.17)
		if isAuthEndpoint(path) {
			if !enforceRateLimit(w, r, authLimiter, "auth", logger) {
				return
			}
		}
		if !enforceRateLimit(w, r, apiLimiter, "api", logger) {
			return
		}

		// Rute za korisnicki servis (Users)
		if isUsersPath(path) {
			usersProxy.ServeHTTP(w, r)
			return
		}

		// Rute za datoteke (Files)
		if isFilesPath(path) {
			filesProxy.ServeHTTP(w, r)
			return
		}

		// Rute za obavestenja (Notifications)
		if isNotificationsPath(path) {
			notifProxy.ServeHTTP(w, r)
			return
		}

		platform.WriteError(w, http.StatusNotFound, "not found")
	})
}
