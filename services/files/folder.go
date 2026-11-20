package main

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Folder struct {
	ID        bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	OwnerID   bson.ObjectID  `bson:"ownerId" json:"ownerId"`
	Name      string         `bson:"name" json:"name"`
	ParentID  *bson.ObjectID `bson:"parentId" json:"parentId"`
	CreatedAt time.Time      `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time      `bson:"updatedAt" json:"updatedAt"`
}
