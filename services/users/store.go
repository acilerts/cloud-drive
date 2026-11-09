package main

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	ErrEmailTaken   = errors.New("email already registered")
	ErrInvalidToken = errors.New("invalid or expired token")
)

type Store struct {
	users  *mongo.Collection
	tokens *mongo.Collection
}

func NewStore(db *mongo.Database) *Store {
	return &Store{users: db.Collection("users"), tokens: db.Collection("tokens")}
}

func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}
	_, err = s.tokens.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "expiresAt", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	})
	if err != nil {
		return err
	}
	_, err = s.tokens.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "tokenHash", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

func (s *Store) CreateUser(ctx context.Context, u *User) error {
	u.ID = bson.NewObjectID()
	_, err := s.users.InsertOne(ctx, u)
	if mongo.IsDuplicateKeyError(err) {
		return ErrEmailTaken
	}
	return err
}

func (s *Store) DeleteUser(ctx context.Context, id bson.ObjectID) error {
	_, err := s.users.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (s *Store) CreateToken(ctx context.Context, userID bson.ObjectID, typ, tokenHash string, ttl time.Duration) error {
	now := time.Now().UTC()
	_, err := s.tokens.InsertOne(ctx, Token{
		ID:        bson.NewObjectID(),
		UserID:    userID,
		Type:      typ,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	})
	return err
}

func (s *Store) ConsumeToken(ctx context.Context, typ, tokenHash string) (bson.ObjectID, error) {
	var t Token
	err := s.tokens.FindOneAndDelete(ctx, bson.M{
		"tokenHash": tokenHash,
		"type":      typ,
		"expiresAt": bson.M{"$gt": time.Now().UTC()},
	}).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return bson.ObjectID{}, ErrInvalidToken
	}
	if err != nil {
		return bson.ObjectID{}, err
	}
	return t.UserID, nil
}

func (s *Store) ActivateUser(ctx context.Context, id bson.ObjectID) error {
	res, err := s.users.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"status": StatusActive}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrInvalidToken
	}
	return nil
}
