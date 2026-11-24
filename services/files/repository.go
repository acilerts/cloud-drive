package main

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Repository struct {
	folders      *mongo.Collection
	files        *mongo.Collection
	fileVersions *mongo.Collection
}

func NewRepository(database *mongo.Database) *Repository {
	return &Repository{
		folders:      database.Collection("folders"),
		files:        database.Collection("files"),
		fileVersions: database.Collection("file_versions"),
	}
}

func (repository *Repository) EnsureIndexes(ctx context.Context) error {
	indexes := []struct {
		collection *mongo.Collection
		model      mongo.IndexModel
		name       string
	}{
		{
			collection: repository.folders,
			model: mongo.IndexModel{
				Keys: bson.D{
					{Key: "ownerId", Value: 1},
					{Key: "parentId", Value: 1},
				},
				Options: options.Index().SetName("owner_parent_idx"),
			},
			name: "folders",
		},
		{
			collection: repository.files,
			model: mongo.IndexModel{
				Keys: bson.D{
					{Key: "ownerId", Value: 1},
					{Key: "parentId", Value: 1},
				},
				Options: options.Index().SetName("owner_parent_idx"),
			},
			name: "files",
		},
		{
			collection: repository.fileVersions,
			model: mongo.IndexModel{
				Keys: bson.D{
					{Key: "fileId", Value: 1},
					{Key: "versionNumber", Value: 1},
				},
				Options: options.Index().SetName("file_version_idx").SetUnique(true),
			},
			name: "file_versions",
		},
	}

	for _, index := range indexes {
		_, err := index.collection.Indexes().CreateOne(ctx, index.model)
		if err != nil {
			return fmt.Errorf("create %s index: %w", index.name, err)
		}
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

func (repository *Repository) CreateFile(
	ctx context.Context,
	file *File,
) error {
	result, err := repository.files.InsertOne(ctx, file)
	if err != nil {
		return fmt.Errorf("insert file: %w", err)
	}

	id, ok := result.InsertedID.(bson.ObjectID)
	if !ok {
		return fmt.Errorf("unexpected inserted file ID type")
	}

	file.ID = id

	return nil
}

func (repository *Repository) CreateFileVersion(
	ctx context.Context,
	version *FileVersion,
) error {
	result, err := repository.fileVersions.InsertOne(ctx, version)
	if err != nil {
		return fmt.Errorf("insert file version: %w", err)
	}

	id, ok := result.InsertedID.(bson.ObjectID)
	if !ok {
		return fmt.Errorf("unexpected inserted file version ID type")
	}

	version.ID = id

	return nil
}

func (repository *Repository) DeleteFile(
	ctx context.Context,
	fileID bson.ObjectID,
	ownerID bson.ObjectID,
) error {
	_, err := repository.files.DeleteOne(ctx, bson.M{
		"_id":     fileID,
		"ownerId": ownerID,
	})

	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}

	return nil
}

func (repository *Repository) DeleteFileAndVersions(
	ctx context.Context,
	fileID bson.ObjectID,
	ownerID bson.ObjectID,
) error {
	var file File

	err := repository.files.FindOne(
		ctx,
		bson.M{
			"_id":     fileID,
			"ownerId": ownerID,
		},
	).Decode(&file)

	if err != nil {
		return fmt.Errorf("find file before deletion: %w", err)
	}

	_, err = repository.fileVersions.DeleteMany(ctx, bson.M{
		"fileId": fileID,
	})
	if err != nil {
		return fmt.Errorf("delete file versions: %w", err)
	}

	result, err := repository.files.DeleteOne(ctx, bson.M{
		"_id":     fileID,
		"ownerId": ownerID,
	})
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}

	if result.DeletedCount != 1 {
		return fmt.Errorf("file was not deleted")
	}

	return nil
}

func (repository *Repository) FindFileByID(
	ctx context.Context,
	fileID bson.ObjectID,
	ownerID bson.ObjectID,
) (*File, error) {
	var file File

	err := repository.files.FindOne(
		ctx,
		bson.M{
			"_id":     fileID,
			"ownerId": ownerID,
		},
	).Decode(&file)

	if err != nil {
		return nil, err
	}

	return &file, nil
}

func (repository *Repository) FindLatestFileVersion(
	ctx context.Context,
	fileID bson.ObjectID,
) (*FileVersion, error) {
	var version FileVersion

	err := repository.fileVersions.FindOne(
		ctx,
		bson.M{"fileId": fileID},
		options.FindOne().SetSort(bson.D{
			{Key: "versionNumber", Value: -1},
		}),
	).Decode(&version)

	if err != nil {
		return nil, err
	}

	return &version, nil
}

func (repository *Repository) UpdateFileVersion(
	ctx context.Context,
	fileID bson.ObjectID,
	ownerID bson.ObjectID,
	expectedVersion int,
	newVersion int,
	updatedAt time.Time,
) error {
	result, err := repository.files.UpdateOne(
		ctx,
		bson.M{
			"_id":            fileID,
			"ownerId":        ownerID,
			"currentVersion": expectedVersion,
		},
		bson.M{
			"$set": bson.M{
				"currentVersion": newVersion,
				"updatedAt":      updatedAt,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("update file version: %w", err)
	}

	if result.MatchedCount != 1 {
		return fmt.Errorf("file version changed concurrently or file not found")
	}

	return nil
}

func (repository *Repository) DeleteFileVersion(
	ctx context.Context,
	fileID bson.ObjectID,
	versionNumber int,
) error {
	_, err := repository.fileVersions.DeleteOne(
		ctx,
		bson.M{
			"fileId":        fileID,
			"versionNumber": versionNumber,
		},
	)
	if err != nil {
		return fmt.Errorf("delete file version: %w", err)
	}

	return nil
}
