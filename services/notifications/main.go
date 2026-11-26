package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/acilerts/cloud-drive/internal/platform"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("notifications service stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	port, err := platform.Required("NOTIFICATIONS_PORT")
	if err != nil {
		return err
	}
	hostsRaw, err := platform.Required("CASSANDRA_HOSTS")
	if err != nil {
		return err
	}
	pubPath, err := platform.Required("JWT_PUBLIC_KEY_FILE")
	if err != nil {
		return err
	}
	internalToken, err := platform.Required("INTERNAL_API_TOKEN")
	if err != nil {
		return err
	}
	if len(internalToken) < 32 {
		return fmt.Errorf("INTERNAL_API_TOKEN must be at least 32 characters")
	}
	keyspace := platform.Default("CASSANDRA_KEYSPACE", "cloud_drive_notifications")

	var hosts []string
	for _, h := range strings.Split(hostsRaw, ",") {
		if h = strings.TrimSpace(h); h != "" {
			hosts = append(hosts, h)
		}
	}
	if len(hosts) == 0 {
		return fmt.Errorf("CASSANDRA_HOSTS is empty")
	}

	jwtPub, err := platform.LoadPublicKey(pubPath)
	if err != nil {
		return fmt.Errorf("load jwt public key: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := OpenStore(ctx, logger, hosts, keyspace)
	if err != nil {
		return err
	}
	defer store.Close()

	app := &Server{store: store, logger: logger, jwtPub: jwtPub, internalToken: internalToken}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", platform.HealthHandler("notifications", store.Ping))
	app.Routes(mux)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("notifications service listening", "port", port)
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
	return nil
}
