package main

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var ErrQuotaExceeded = errors.New("quota exceeded")

func (s *Store) ReserveQuota(ctx context.Context, userID bson.ObjectID, bytes int64) (*QuotaReservation, error) {
	if bytes <= 0 {
		return nil, ErrQuotaExceeded
	}

	filter := bson.M{
		"_id":    userID,
		"status": StatusActive,
		"$expr": bson.M{
			"$lte": bson.A{
				bson.M{
					"$add": bson.A{
						bson.M{"$ifNull": bson.A{"$usedBytes", 0}},
						bson.M{"$ifNull": bson.A{"$reservedBytes", 0}},
						bytes,
					},
				},
				"$quotaBytes",
			},
		},
	}

	var user User
	err := s.users.FindOneAndUpdate(
		ctx,
		filter,
		bson.M{"$inc": bson.M{"reservedBytes": bytes}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&user)

	if errors.Is(err, mongo.ErrNoDocuments) {
		existing, findErr := s.FindUserByID(ctx, userID)
		if findErr != nil {
			return nil, findErr
		}
		if existing.Status != StatusActive {
			return nil, ErrNotFound
		}
		return nil, ErrQuotaExceeded
	}
	if err != nil {
		return nil, err
	}

	reservation := &QuotaReservation{
		ID:        bson.NewObjectID(),
		UserID:    userID,
		Bytes:     bytes,
		Status:    ReservationReserved,
		CreatedAt: time.Now().UTC(),
	}

	if _, err := s.quotaReservations().InsertOne(ctx, reservation); err != nil {
		_, rollbackErr := s.users.UpdateOne(
			ctx,
			bson.M{"_id": userID},
			bson.M{"$inc": bson.M{"reservedBytes": -bytes}},
		)
		if rollbackErr != nil {
			return nil, errors.Join(err, rollbackErr)
		}
		return nil, err
	}

	return reservation, nil
}

var (
	ErrReservationNotFound = errors.New("quota reservation not found")
	ErrReservationInactive = errors.New("quota reservation is not active")
)

func (s *Store) CommitQuota(ctx context.Context, userID, reservationID bson.ObjectID) error {
	var reservation QuotaReservation

	err := s.quotaReservations().FindOne(
		ctx,
		bson.M{
			"_id":    reservationID,
			"userId": userID,
		},
	).Decode(&reservation)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrReservationNotFound
	}
	if err != nil {
		return err
	}

	if reservation.Status == ReservationCommitted {
		return nil
	}
	if reservation.Status != ReservationReserved {
		return ErrReservationInactive
	}

	result, err := s.quotaReservations().UpdateOne(
		ctx,
		bson.M{
			"_id":    reservationID,
			"userId": userID,
			"status": ReservationReserved,
		},
		bson.M{
			"$set": bson.M{"status": ReservationCommitted},
		},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrReservationInactive
	}

	userResult, err := s.users.UpdateOne(
		ctx,
		bson.M{
			"_id":           userID,
			"reservedBytes": bson.M{"$gte": reservation.Bytes},
		},
		bson.M{
			"$inc": bson.M{
				"reservedBytes": -reservation.Bytes,
				"usedBytes":     reservation.Bytes,
			},
		},
	)

	if err != nil || userResult.MatchedCount == 0 {
		_, rollbackErr := s.quotaReservations().UpdateOne(
			ctx,
			bson.M{
				"_id":    reservationID,
				"userId": userID,
				"status": ReservationCommitted,
			},
			bson.M{
				"$set": bson.M{"status": ReservationReserved},
			},
		)

		if err != nil {
			return errors.Join(err, rollbackErr)
		}

		return errors.Join(ErrReservationInactive, rollbackErr)
	}

	return nil
}

func (s *Store) ReleaseQuota(ctx context.Context, userID, reservationID bson.ObjectID) error {
	var reservation QuotaReservation

	err := s.quotaReservations().FindOne(
		ctx,
		bson.M{
			"_id":    reservationID,
			"userId": userID,
		},
	).Decode(&reservation)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrReservationNotFound
	}
	if err != nil {
		return err
	}

	if reservation.Status == ReservationReleased {
		return nil
	}
	if reservation.Status != ReservationReserved {
		return ErrReservationInactive
	}

	result, err := s.quotaReservations().UpdateOne(
		ctx,
		bson.M{
			"_id":    reservationID,
			"userId": userID,
			"status": ReservationReserved,
		},
		bson.M{
			"$set": bson.M{"status": ReservationReleased},
		},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrReservationInactive
	}

	userResult, err := s.users.UpdateOne(
		ctx,
		bson.M{
			"_id":           userID,
			"reservedBytes": bson.M{"$gte": reservation.Bytes},
		},
		bson.M{
			"$inc": bson.M{"reservedBytes": -reservation.Bytes},
		},
	)

	if err != nil || userResult.MatchedCount == 0 {
		_, rollbackErr := s.quotaReservations().UpdateOne(
			ctx,
			bson.M{
				"_id":    reservationID,
				"userId": userID,
				"status": ReservationReleased,
			},
			bson.M{
				"$set": bson.M{"status": ReservationReserved},
			},
		)

		if err != nil {
			return errors.Join(err, rollbackErr)
		}

		return errors.Join(ErrReservationInactive, rollbackErr)
	}

	return nil
}

func (s *Store) GetQuotaReservationStatus(
	ctx context.Context,
	userID, reservationID bson.ObjectID,
) (string, error) {
	var reservation QuotaReservation

	err := s.quotaReservations().FindOne(
		ctx,
		bson.M{
			"_id":    reservationID,
			"userId": userID,
		},
	).Decode(&reservation)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", ErrReservationNotFound
	}
	if err != nil {
		return "", err
	}

	return reservation.Status, nil
}
