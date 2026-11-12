package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("gateway service stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	port := platform.Default("GATEWAY_PORT", "8080")
	usersRaw := platform.Default("USERS_SERVICE_URL", "http://users:8081")
	filesRaw := platform.Default("FILES_SERVICE_URL", "http://files:8082")
	notifRaw := platform.Default("NOTIFICATIONS_SERVICE_URL", "http://notifications:8084")
	clientDir := platform.Default("CLIENT_DIR", "/client")

	usersURL, err := url.Parse(usersRaw)
	if err != nil {
		return fmt.Errorf("invalid USERS_SERVICE_URL: %w", err)
	}
	filesURL, _ := url.Parse(filesRaw)
	notifURL, _ := url.Parse(notifRaw)

	cfg := &Config{
		Port:             port,
		UsersURL:         usersURL,
		FilesURL:         filesURL,
		NotificationsURL: notifURL,
		ClientDir:        clientDir,
	}

	handler := newGatewayHandler(cfg, logger)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api gateway listening", "port", cfg.Port)
		serverErr <- srv.ListenAndServe()
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

	fileServer := http.FileServer(http.Dir(cfg.ClientDir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
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

		// Pocetna stranica
		if path == "/" {
			http.Redirect(w, r, "/login.html", http.StatusFound)
			return
		}

		// Serviranje klijentskih statickih fajlova ako fajl postoji
		cleanPath := filepath.Clean(filepath.Join(cfg.ClientDir, path))
		if info, err := os.Stat(cleanPath); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Rute za korisnicki servis (Users)
		if strings.HasPrefix(path, "/register") ||
			strings.HasPrefix(path, "/confirm") ||
			strings.HasPrefix(path, "/login") ||
			strings.HasPrefix(path, "/password-") ||
			strings.HasPrefix(path, "/users") {
			usersProxy.ServeHTTP(w, r)
			return
		}

		// Rute za datoteke (Files)
		if strings.HasPrefix(path, "/files") || strings.HasPrefix(path, "/folders") {
			filesProxy.ServeHTTP(w, r)
			return
		}

		// Rute za obavestenja (Notifications)
		if strings.HasPrefix(path, "/notifications") {
			notifProxy.ServeHTTP(w, r)
			return
		}

		fileServer.ServeHTTP(w, r)
	})
}
