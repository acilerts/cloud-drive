package main

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Repository struct {
	folders *mongo.Collection
}

func NewRepository(database *mongo.Database) *Repository {
	return &Repository{
		folders: database.Collection("folders"),
	}
}

func (repository *Repository) EnsureIndexes(ctx context.Context) error {
	_, err := repository.folders.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys: bson.D{
				{Key: "ownerId", Value: 1},
				{Key: "parentId", Value: 1},
			},
			Options: options.Index().SetName("owner_parent_idx"),
		},
	)
	if err != nil {
		return fmt.Errorf("create folders index: %w", err)
	}

	return nil
}

func (repository *Repository) CreateFolder(
	ctx context.Context,
	folder *Folder,
) error {
	result, err := repository.folders.InsertOne(ctx, folder)
	if err != nil {
		return fmt.Errorf("insert folder: %w", err)
	}

	id, ok := result.InsertedID.(bson.ObjectID)
	if !ok {
		return fmt.Errorf("unexpected inserted folder ID type")
	}

	folder.ID = id

	return nil
}

func (repository *Repository) FindFolderByID(
	ctx context.Context,
	folderID bson.ObjectID,
	ownerID bson.ObjectID,
) (*Folder, error) {
	var folder Folder

	err := repository.folders.FindOne(
		ctx,
		bson.M{
			"_id":     folderID,
			"ownerId": ownerID,
		},
	).Decode(&folder)

	if err != nil {
		return nil, err
	}

	return &folder, nil
}

func (repository *Repository) FindFolders(
	ctx context.Context,
	ownerID bson.ObjectID,
	parentID *bson.ObjectID,
) ([]Folder, error) {
	filter := bson.M{
		"ownerId":  ownerID,
		"parentId": parentID,
	}

	cursor, err := repository.folders.Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{
			{Key: "name", Value: 1},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("find folders: %w", err)
	}
	defer cursor.Close(ctx)

	var folders []Folder

	if err := cursor.All(ctx, &folders); err != nil {
		return nil, fmt.Errorf("decode folders: %w", err)
	}

	if folders == nil {
		folders = []Folder{}
	}

	return folders, nil
}
