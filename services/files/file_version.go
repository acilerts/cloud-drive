package main

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type FileVersion struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	FileID        bson.ObjectID `bson:"fileId" json:"fileId"`
	VersionNumber int           `bson:"versionNumber" json:"versionNumber"`
	ObjectKey     string        `bson:"objectKey" json:"objectKey"`
	SizeBytes     int64         `bson:"sizeBytes" json:"sizeBytes"`
	ContentType   string        `bson:"contentType" json:"contentType"`
	CreatedAt     time.Time     `bson:"createdAt" json:"createdAt"`
}
