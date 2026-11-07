package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/acilerts/cloud-drive/internal/platform"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("users service stopped", "error", err)
		os.Exit(1)
	}
}
func run(logger *slog.Logger) error {
	port, err := platform.Required("USERS_PORT")
	if err != nil {
		return err
	}
	mongoURI, err := platform.Required("MONGO_URI")
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		return fmt.Errorf("mongo connect: %w", err)
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Disconnect(c)
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", platform.HealthHandler("users", func(ctx context.Context) error {
		return client.Ping(ctx, readpref.Primary())
	}))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("users service listening", "port", port)
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
