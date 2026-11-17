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

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/acilerts/cloud-drive/internal/platform"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("files service stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	port, err := platform.Required("FILES_PORT")
	if err != nil {
		return err
	}

	mongoURI, err := platform.Required("MONGO_URI")
	if err != nil {
		return err
	}

	publicKeyPath, err := platform.Required("JWT_PUBLIC_KEY_FILE")
	if err != nil {
		return err
	}

	dbName := platform.Default("FILES_MONGO_DB", "files")

	publicKey, err := platform.LoadPublicKey(publicKeyPath)
	if err != nil {
		return fmt.Errorf("load jwt public key: %w", err)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	client, err := mongo.Connect(
		options.Client().ApplyURI(mongoURI),
	)
	if err != nil {
		return fmt.Errorf("mongo connect: %w", err)
	}

	defer func() {
		disconnectCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		_ = client.Disconnect(disconnectCtx)
	}()

	pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
	defer cancelPing()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		return fmt.Errorf("mongo ping: %w", err)
	}

	repository := NewRepository(client.Database(dbName))

	indexCtx, cancelIndex := context.WithTimeout(ctx, 10*time.Second)
	defer cancelIndex()

	if err := repository.EnsureIndexes(indexCtx); err != nil {
		return fmt.Errorf("ensure indexes: %w", err)
	}

	service := NewService(repository)
	handler := NewHandler(service, publicKey)

	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /health",
		platform.HealthHandler("files", func(ctx context.Context) error {
			return client.Ping(ctx, readpref.Primary())
		}),
	)

	handler.Routes(mux)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverError := make(chan error, 1)

	go func() {
		logger.Info("files service listening", "port", port)
		serverError <- server.ListenAndServe()
	}()

	select {
	case err := <-serverError:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}

	case <-ctx.Done():
		logger.Info("shutting down files service")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown files server: %w", err)
	}

	return nil
}
