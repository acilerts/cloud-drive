package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	ErrEmailTaken   = errors.New("email already registered")
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrNotFound     = errors.New("not found")
)

type Store struct {
	users  *mongo.Collection
	tokens *mongo.Collection
	otps   *mongo.Collection
}

func NewStore(db *mongo.Database) *Store {
	return &Store{users: db.Collection("users"), tokens: db.Collection("tokens"), otps: db.Collection("otps")}
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
	if err != nil {
		return err
	}
	_, err = s.otps.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "userId", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}
	_, err = s.otps.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "expiresAt", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
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

func (s *Store) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := s.users.FindOne(ctx, bson.M{"email": email}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
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

func (s *Store) RecordFailedLogin(ctx context.Context, id bson.ObjectID, maxFailures int, lockFor time.Duration) (bool, error) {
	var u User
	err := s.users.FindOneAndUpdate(ctx,
		bson.M{"_id": id},
		bson.M{"$inc": bson.M{"failedLogins": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&u)
	if err != nil {
		return false, err
	}
	if u.FailedLogins < maxFailures {
		return false, nil
	}
	_, err = s.users.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{
		"lockedUntil":  time.Now().UTC().Add(lockFor),
		"failedLogins": 0,
	}})
	return true, err
}

func (s *Store) ClearFailedLogins(ctx context.Context, id bson.ObjectID) error {
	_, err := s.users.UpdateOne(ctx, bson.M{"_id": id},
		bson.M{"$set": bson.M{"failedLogins": 0}})
	return err
}

func (s *Store) SaveOTP(ctx context.Context, userID bson.ObjectID, codeHash string, ttl time.Duration) error {
	now := time.Now().UTC()
	_, err := s.otps.UpdateOne(ctx,
		bson.M{"userId": userID},
		bson.M{"$set": bson.M{
			"codeHash":  codeHash,
			"attempts":  0,
			"expiresAt": now.Add(ttl),
			"createdAt": now,
		}},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}

func (s *Store) VerifyOTP(ctx context.Context, userID bson.ObjectID, codeHash string, maxAttempts int) error {
	var o OTP
	err := s.otps.FindOneAndUpdate(ctx,
		bson.M{
			"userId":    userID,
			"expiresAt": bson.M{"$gt": time.Now().UTC()},
			"attempts":  bson.M{"$lt": maxAttempts},
		},
		bson.M{"$inc": bson.M{"attempts": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&o)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrInvalidToken
	}
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(o.CodeHash), []byte(codeHash)) != 1 {
		return ErrInvalidToken
	}
	res, err := s.otps.DeleteOne(ctx, bson.M{"_id": o.ID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrInvalidToken
	}
	return nil
}
