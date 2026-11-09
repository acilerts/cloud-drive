package main

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"

	StatusPending = "pending"
	StatusActive  = "active"
)

type User struct {
	ID                bson.ObjectID `bson:"_id"`
	FirstName         string        `bson:"firstName"`
	LastName          string        `bson:"lastName"`
	Email             string        `bson:"email"`
	PasswordHash      string        `bson:"passwordHash"`
	Role              string        `bson:"role"`
	Status            string        `bson:"status"`
	QuotaBytes        int64         `bson:"quotaBytes"`
	PasswordChangedAt time.Time     `bson:"passwordChangedAt"`
	CreatedAt         time.Time     `bson:"createdAt"`
}

type Token struct {
	ID        bson.ObjectID `bson:"_id"`
	UserID    bson.ObjectID `bson:"userId"`
	Type      string        `bson:"type"`
	TokenHash string        `bson:"tokenHash"`
	ExpiresAt time.Time     `bson:"expiresAt"`
	CreatedAt time.Time     `bson:"createdAt"`
}

const TokenConfirm = "confirm"
