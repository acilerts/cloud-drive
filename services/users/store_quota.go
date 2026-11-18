package main

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var ErrPendingExists = errors.New("pending quota request already exists")

func (s *Store) quotaReqs() *mongo.Collection {
	return s.users.Database().Collection("quotaRequests")
}

func (s *Store) EnsureQuotaIndexes(ctx context.Context) error {
	_, err := s.quotaReqs().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "userId", Value: 1}},
		Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"status": QuotaPending}),
	})
	return err
}

func (s *Store) CreateQuotaRequest(ctx context.Context, qr *QuotaRequest) error {
	qr.ID = bson.NewObjectID()
	_, err := s.quotaReqs().InsertOne(ctx, qr)
	if mongo.IsDuplicateKeyError(err) {
		return ErrPendingExists
	}
	return err
}

func (s *Store) listQuotaRequests(ctx context.Context, filter bson.M) ([]QuotaRequest, error) {
	cur, err := s.quotaReqs().Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(200))
	if err != nil {
		return nil, err
	}
	var out []QuotaRequest
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListQuotaRequestsByUser(ctx context.Context, userID bson.ObjectID) ([]QuotaRequest, error) {
	return s.listQuotaRequests(ctx, bson.M{"userId": userID})
}

func (s *Store) ListQuotaRequestsByStatus(ctx context.Context, status string) ([]QuotaRequest, error) {
	return s.listQuotaRequests(ctx, bson.M{"status": status})
}

func (s *Store) DecideQuotaRequest(ctx context.Context, id, adminID bson.ObjectID, status string) (*QuotaRequest, error) {
	var qr QuotaRequest
	err := s.quotaReqs().FindOneAndUpdate(ctx,
		bson.M{"_id": id, "status": QuotaPending},
		bson.M{"$set": bson.M{
			"status":    status,
			"decidedAt": time.Now().UTC(),
			"decidedBy": adminID,
		}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&qr)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &qr, nil
}

func (s *Store) ReopenQuotaRequest(ctx context.Context, id bson.ObjectID) error {
	_, err := s.quotaReqs().UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set":   bson.M{"status": QuotaPending},
		"$unset": bson.M{"decidedAt": "", "decidedBy": ""},
	})
	return err
}

func (s *Store) SetQuota(ctx context.Context, userID bson.ObjectID, bytes int64) error {
	res, err := s.users.UpdateOne(ctx, bson.M{"_id": userID},
		bson.M{"$set": bson.M{"quotaBytes": bytes}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	cur, err := s.users.Find(ctx, bson.M{},
		options.Find().SetSort(bson.D{{Key: "email", Value: 1}}).SetLimit(1000))
	if err != nil {
		return nil, err
	}
	var out []User
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) FindUsersByIDs(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID]User, error) {
	result := map[bson.ObjectID]User{}
	if len(ids) == 0 {
		return result, nil
	}
	cur, err := s.users.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	var users []User
	if err := cur.All(ctx, &users); err != nil {
		return nil, err
	}
	for _, u := range users {
		result[u.ID] = u
	}
	return result, nil
}
