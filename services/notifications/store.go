package main

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/gocql/gocql"
)

var keyspacePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,47}$`)

type Notification struct {
	UserID     string
	ID         gocql.UUID
	Kind       string
	Title      string
	Message    string
	ResourceID string
}

type Store struct {
	session *gocql.Session
}

func OpenStore(ctx context.Context, logger *slog.Logger, hosts []string, keyspace string) (*Store, error) {
	if !keyspacePattern.MatchString(keyspace) {
		return nil, fmt.Errorf("invalid CASSANDRA_KEYSPACE")
	}
	newCluster := func() *gocql.ClusterConfig {
		c := gocql.NewCluster(hosts...)
		c.ProtoVersion = 4
		c.Consistency = gocql.One
		c.Timeout = 5 * time.Second
		c.ConnectTimeout = 5 * time.Second
		return c
	}

	var admin *gocql.Session
	var err error
	for attempt := 1; attempt <= 30; attempt++ {
		admin, err = newCluster().CreateSession()
		if err == nil {
			break
		}
		logger.Warn("cassandra not ready, retrying", "attempt", attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
	if err != nil {
		return nil, fmt.Errorf("connect to cassandra: %w", err)
	}
	err = admin.Query(`CREATE KEYSPACE IF NOT EXISTS ` + keyspace +
		` WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}`).Exec()
	admin.Close()
	if err != nil {
		return nil, fmt.Errorf("create keyspace: %w", err)
	}

	cluster := newCluster()
	cluster.Keyspace = keyspace
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("open keyspace session: %w", err)
	}

	err = session.Query(`CREATE TABLE IF NOT EXISTS notifications_by_user (
		user_id     text,
		id          timeuuid,
		kind        text,
		title       text,
		message     text,
		resource_id text,
		PRIMARY KEY ((user_id), id)
	) WITH CLUSTERING ORDER BY (id DESC)`).Exec()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}
	return &Store{session: session}, nil
}

func (s *Store) Close() { s.session.Close() }

func (s *Store) Ping(ctx context.Context) error {
	return s.session.Query(`SELECT now() FROM system.local`).WithContext(ctx).Exec()
}

func (s *Store) Create(ctx context.Context, n *Notification) error {
	n.ID = gocql.TimeUUID()
	return s.session.Query(
		`INSERT INTO notifications_by_user (user_id, id, kind, title, message, resource_id)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		n.UserID, n.ID, n.Kind, n.Title, n.Message, n.ResourceID,
	).WithContext(ctx).Exec()
}

func (s *Store) ListByUser(ctx context.Context, userID string, limit int) ([]Notification, error) {
	iter := s.session.Query(
		`SELECT id, kind, title, message, resource_id
		 FROM notifications_by_user WHERE user_id = ? LIMIT ?`,
		userID, limit,
	).WithContext(ctx).Iter()

	out := []Notification{}
	n := Notification{UserID: userID}
	for iter.Scan(&n.ID, &n.Kind, &n.Title, &n.Message, &n.ResourceID) {
		out = append(out, n)
		n = Notification{UserID: userID}
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return out, nil
}
