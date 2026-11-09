package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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
	smtpHost, err := platform.Required("SMTP_HOST")
	if err != nil {
		return err
	}
	clientBaseURL, err := platform.Required("CLIENT_BASE_URL")
	if err != nil {
		return err
	}
	dbName := platform.Default("MONGO_DB", "users")
	smtpPort := platform.Default("SMTP_PORT", "1025")
	mailFrom := platform.Default("MAIL_FROM", "no-reply@cloud-drive.local")

	quota, err := strconv.ParseInt(platform.Default("DEFAULT_QUOTA_BYTES", "1073741824"), 10, 64)
	if err != nil || quota <= 0 {
		return fmt.Errorf("invalid DEFAULT_QUOTA_BYTES")
	}
	confirmTTL, err := time.ParseDuration(platform.Default("CONFIRM_TTL", "24h"))
	if err != nil || confirmTTL <= 0 {
		return fmt.Errorf("invalid CONFIRM_TTL")
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

	store := NewStore(client.Database(dbName))
	idxCtx, cancelIdx := context.WithTimeout(ctx, 10*time.Second)
	defer cancelIdx()
	if err := store.EnsureIndexes(idxCtx); err != nil {
		return fmt.Errorf("ensure indexes: %w", err)
	}

	app := &Server{
		store:         store,
		mailer:        &Mailer{host: smtpHost, port: smtpPort, from: mailFrom},
		logger:        logger,
		clientBaseURL: clientBaseURL,
		defaultQuota:  quota,
		confirmTTL:    confirmTTL,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", platform.HealthHandler("users", func(ctx context.Context) error {
		return client.Ping(ctx, readpref.Primary())
	}))
	app.Routes(mux)

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
