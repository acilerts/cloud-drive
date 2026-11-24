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
	UsedBytes         int64         `bson:"usedBytes"`
	ReservedBytes     int64         `bson:"reservedBytes"`
	PasswordChangedAt time.Time     `bson:"passwordChangedAt"`
	CreatedAt         time.Time     `bson:"createdAt"`
	FailedLogins      int           `bson:"failedLogins"`
	LockedUntil       time.Time     `bson:"lockedUntil"`
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

type OTP struct {
	ID        bson.ObjectID `bson:"_id"`
	UserID    bson.ObjectID `bson:"userId"`
	CodeHash  string        `bson:"codeHash"`
	Attempts  int           `bson:"attempts"`
	ExpiresAt time.Time     `bson:"expiresAt"`
	CreatedAt time.Time     `bson:"createdAt"`
}

type QuotaReservation struct {
	ID        bson.ObjectID `bson:"_id" json:"reservationId"`
	UserID    bson.ObjectID `bson:"userId" json:"-"`
	Bytes     int64         `bson:"bytes" json:"bytes"`
	Status    string        `bson:"status" json:"status"`
	CreatedAt time.Time     `bson:"createdAt" json:"createdAt"`
}

const (
	ReservationReserved  = "reserved"
	ReservationCommitted = "committed"
	ReservationReleased  = "released"
)
