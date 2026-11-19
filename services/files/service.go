package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var (
	ErrInvalidFolderName = errors.New("invalid folder name")
	ErrParentNotFound    = errors.New("parent folder not found")
)

type folderRepository interface {
	CreateFolder(ctx context.Context, folder *Folder) error
	FindFolderByID(
		ctx context.Context,
		folderID bson.ObjectID,
		ownerID bson.ObjectID,
	) (*Folder, error)
}

type FolderRepository interface {
	CreateFolder(ctx context.Context, folder *Folder) error

	FindFolderByID(
		ctx context.Context,
		folderID bson.ObjectID,
		ownerID bson.ObjectID,
	) (*Folder, error)
}

type Service struct {
	repository FolderRepository
}

func NewService(repository FolderRepository) *Service {
	return &Service{
		repository: repository,
	}
}

func (service *Service) CreateFolder(
	ctx context.Context,
	ownerID bson.ObjectID,
	name string,
	parentID *bson.ObjectID,
) (*Folder, error) {
	name = strings.TrimSpace(name)

	if name == "" {
		return nil, ErrInvalidFolderName
	}

	if ownerID.IsZero() {
		return nil, errors.New("invalid folder owner")
	}

	if parentID != nil {
		_, err := service.repository.FindFolderByID(
			ctx,
			*parentID,
			ownerID,
		)

		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, ErrParentNotFound
			}

			return nil, err
		}
	}

	now := time.Now().UTC()

	folder := &Folder{
		ID:        bson.NewObjectID(),
		OwnerID:   ownerID,
		Name:      name,
		ParentID:  parentID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := service.repository.CreateFolder(ctx, folder); err != nil {
		return nil, err
	}

	return folder, nil
}
