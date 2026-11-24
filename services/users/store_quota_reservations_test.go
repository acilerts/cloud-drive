package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func newQuotaTestStore(t *testing.T) *Store {
	t.Helper()

	uri := os.Getenv("USERS_TEST_MONGO_URI")
	dbPrefix := os.Getenv("USERS_TEST_MONGO_DB")
	if uri == "" || dbPrefix == "" {
		t.Skip("USERS_TEST_MONGO_URI and USERS_TEST_MONGO_DB must be set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect to test MongoDB: %v", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		t.Fatalf("ping test MongoDB: %v", err)
	}

	dbName := fmt.Sprintf("%s_%s", dbPrefix, bson.NewObjectID().Hex())
	db := client.Database(dbName)

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()

		if err := db.Drop(cleanupCtx); err != nil {
			t.Errorf("drop test database: %v", err)
		}
		if err := client.Disconnect(cleanupCtx); err != nil {
			t.Errorf("disconnect test MongoDB: %v", err)
		}
	})

	store := NewStore(db)
	if err := store.EnsureQuotaReservationIndexes(ctx); err != nil {
		t.Fatalf("create quota reservation indexes: %v", err)
	}

	return store
}

func createQuotaTestUser(
	t *testing.T,
	store *Store,
	quota, used int64,
	status string,
) bson.ObjectID {
	t.Helper()

	id := bson.NewObjectID()

	_, err := store.users.InsertOne(context.Background(), User{
		ID:            id,
		Email:         id.Hex() + "@example.test",
		Status:        status,
		QuotaBytes:    quota,
		UsedBytes:     used,
		ReservedBytes: 0,
	})
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}

	return id
}

func TestReserveQuota(t *testing.T) {
	store := newQuotaTestStore(t)
	ctx := context.Background()
	userID := createQuotaTestUser(t, store, 1000, 100, StatusActive)

	reservation, err := store.ReserveQuota(ctx, userID, 400)
	if err != nil {
		t.Fatalf("ReserveQuota returned error: %v", err)
	}

	if reservation.Status != ReservationReserved {
		t.Fatalf("expected status %q, got %q", ReservationReserved, reservation.Status)
	}
	if reservation.Bytes != 400 {
		t.Fatalf("expected 400 reserved bytes, got %d", reservation.Bytes)
	}

	user, err := store.FindUserByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindUserByID returned error: %v", err)
	}
	if user.ReservedBytes != 400 {
		t.Fatalf("expected ReservedBytes=400, got %d", user.ReservedBytes)
	}
}

func TestCommitQuota(t *testing.T) {
	store := newQuotaTestStore(t)
	ctx := context.Background()
	userID := createQuotaTestUser(t, store, 1000, 100, StatusActive)

	reservation, err := store.ReserveQuota(ctx, userID, 400)
	if err != nil {
		t.Fatalf("ReserveQuota returned error: %v", err)
	}

	if err := store.CommitQuota(ctx, userID, reservation.ID); err != nil {
		t.Fatalf("CommitQuota returned error: %v", err)
	}

	user, err := store.FindUserByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindUserByID returned error: %v", err)
	}
	if user.UsedBytes != 500 {
		t.Fatalf("expected UsedBytes=500, got %d", user.UsedBytes)
	}
	if user.ReservedBytes != 0 {
		t.Fatalf("expected ReservedBytes=0, got %d", user.ReservedBytes)
	}

	if err := store.CommitQuota(ctx, userID, reservation.ID); err != nil {
		t.Fatalf("repeated CommitQuota should be idempotent, got: %v", err)
	}
}

func TestReleaseQuota(t *testing.T) {
	store := newQuotaTestStore(t)
	ctx := context.Background()
	userID := createQuotaTestUser(t, store, 1000, 100, StatusActive)

	reservation, err := store.ReserveQuota(ctx, userID, 300)
	if err != nil {
		t.Fatalf("ReserveQuota returned error: %v", err)
	}

	if err := store.ReleaseQuota(ctx, userID, reservation.ID); err != nil {
		t.Fatalf("ReleaseQuota returned error: %v", err)
	}

	user, err := store.FindUserByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindUserByID returned error: %v", err)
	}
	if user.ReservedBytes != 0 {
		t.Fatalf("expected ReservedBytes=0, got %d", user.ReservedBytes)
	}

	if err := store.ReleaseQuota(ctx, userID, reservation.ID); err != nil {
		t.Fatalf("repeated ReleaseQuota should be idempotent, got: %v", err)
	}
}

func TestReserveQuotaExceeded(t *testing.T) {
	store := newQuotaTestStore(t)
	ctx := context.Background()
	userID := createQuotaTestUser(t, store, 1000, 800, StatusActive)

	_, err := store.ReserveQuota(ctx, userID, 201)
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}

	user, err := store.FindUserByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindUserByID returned error: %v", err)
	}
	if user.ReservedBytes != 0 {
		t.Fatalf("expected ReservedBytes=0 after rejected reservation, got %d", user.ReservedBytes)
	}
}

func TestReserveQuotaRejectsInactiveUser(t *testing.T) {
	store := newQuotaTestStore(t)
	ctx := context.Background()
	userID := createQuotaTestUser(t, store, 1000, 0, StatusPending)

	_, err := store.ReserveQuota(ctx, userID, 100)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestConcurrentReserveQuotaDoesNotExceedQuota(t *testing.T) {
	store := newQuotaTestStore(t)
	ctx := context.Background()

	userID := createQuotaTestUser(t, store, 1000, 0, StatusActive)

	const requests = 10
	const reservationSize = 200

	var wg sync.WaitGroup
	var mu sync.Mutex

	successCount := 0
	var errorsList []error

	for i := 0; i < requests; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			_, err := store.ReserveQuota(ctx, userID, reservationSize)

			mu.Lock()
			defer mu.Unlock()

			if err == nil {
				successCount++
			} else {
				errorsList = append(errorsList, err)
			}
		}()
	}

	wg.Wait()

	if successCount != 5 {
		t.Fatalf("expected exactly 5 successful reservations, got %d", successCount)
	}

	for _, err := range errorsList {
		if !errors.Is(err, ErrQuotaExceeded) {
			t.Fatalf("expected ErrQuotaExceeded, got %v", err)
		}
	}

	user, err := store.FindUserByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindUserByID returned error: %v", err)
	}

	if user.ReservedBytes != 1000 {
		t.Fatalf("expected ReservedBytes=1000, got %d", user.ReservedBytes)
	}
}
