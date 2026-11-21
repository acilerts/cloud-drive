package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var (
	ErrInvalidFolderName = errors.New("invalid folder name")
	ErrParentNotFound    = errors.New("parent folder not found")
	ErrInvalidFileName   = errors.New("invalid file name")
	ErrInvalidFileSize   = errors.New("invalid file size")
	ErrInvalidObjectKey  = errors.New("invalid object key")
)

type FolderRepository interface {
	CreateFolder(ctx context.Context, folder *Folder) error

	FindFolderByID(
		ctx context.Context,
		folderID bson.ObjectID,
		ownerID bson.ObjectID,
	) (*Folder, error)

	FindFolders(
		ctx context.Context,
		ownerID bson.ObjectID,
		parentID *bson.ObjectID,
	) ([]Folder, error)

	CreateFile(ctx context.Context, file *File) error

	CreateFileVersion(ctx context.Context, version *FileVersion) error

	DeleteFile(
		ctx context.Context,
		fileID bson.ObjectID,
		ownerID bson.ObjectID,
	) error

	DeleteFileAndVersions(
		ctx context.Context,
		fileID bson.ObjectID,
		ownerID bson.ObjectID,
	) error
}

type ObjectStorage interface {
	Upload(
		ctx context.Context,
		objectKey string,
		reader io.Reader,
		size int64,
		contentType string,
	) error

	Delete(ctx context.Context, objectKey string) error
}

type Service struct {
	repository   FolderRepository
	objectStore  ObjectStorage
	quotaManager QuotaManager
}

func NewService(repository FolderRepository) *Service {
	return NewServiceWithDependencies(repository, nil, nil)
}

func NewServiceWithObjectStore(
	repository FolderRepository,
	objectStore ObjectStorage,
) *Service {
	return NewServiceWithDependencies(repository, objectStore, nil)
}

func NewServiceWithDependencies(
	repository FolderRepository,
	objectStore ObjectStorage,
	quotaManager QuotaManager,
) *Service {
	return &Service{
		repository:   repository,
		objectStore:  objectStore,
		quotaManager: quotaManager,
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

func (service *Service) ListFolders(
	ctx context.Context,
	ownerID bson.ObjectID,
	parentID *bson.ObjectID,
) ([]Folder, error) {
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

	return service.repository.FindFolders(
		ctx,
		ownerID,
		parentID,
	)
}

func (service *Service) RegisterUploadedFile(
	ctx context.Context,
	ownerID bson.ObjectID,
	name string,
	parentID *bson.ObjectID,
	objectKey string,
	sizeBytes int64,
	contentType string,
) (*File, error) {
	name = strings.TrimSpace(name)
	objectKey = strings.TrimSpace(objectKey)

	if ownerID.IsZero() {
		return nil, errors.New("invalid file owner")
	}

	if name == "" {
		return nil, ErrInvalidFileName
	}

	if sizeBytes < 0 {
		return nil, ErrInvalidFileSize
	}

	if objectKey == "" {
		return nil, ErrInvalidObjectKey
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

	file := &File{
		ID:             bson.NewObjectID(),
		OwnerID:        ownerID,
		Name:           name,
		ParentID:       parentID,
		CurrentVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	version := &FileVersion{
		ID:            bson.NewObjectID(),
		FileID:        file.ID,
		VersionNumber: 1,
		ObjectKey:     objectKey,
		SizeBytes:     sizeBytes,
		ContentType:   contentType,
		CreatedAt:     now,
	}

	if err := service.repository.CreateFile(ctx, file); err != nil {
		return nil, err
	}

	if err := service.repository.CreateFileVersion(ctx, version); err != nil {
		cleanupErr := service.repository.DeleteFileAndVersions(
			ctx,
			file.ID,
			ownerID,
		)

		if cleanupErr != nil {
			return nil, errors.Join(
				err,
				fmt.Errorf("cleanup incomplete file record: %w", cleanupErr),
			)
		}

		return nil, err
	}

	return file, nil
}

func (service *Service) UploadFile(
	ctx context.Context,
	ownerID bson.ObjectID,
	name string,
	parentID *bson.ObjectID,
	objectKey string,
	reader io.Reader,
	sizeBytes int64,
	contentType string,
) (*File, error) {
	if service.objectStore == nil {
		return nil, errors.New("object storage is not configured")
	}

	if service.quotaManager == nil {
		return nil, errors.New("quota manager is not configured")
	}

	if sizeBytes < 0 {
		return nil, ErrInvalidFileSize
	}

	userID := ownerID.Hex()
	reservationID := ""

	if sizeBytes > 0 {
		reservation, err := service.quotaManager.Reserve(
			ctx,
			userID,
			sizeBytes,
		)
		if err != nil {
			return nil, fmt.Errorf("reserve storage quota: %w", err)
		}

		reservationID = reservation
	}

	releaseReservation := func() error {
		if reservationID == "" {
			return nil
		}

		if err := service.quotaManager.Release(
			ctx,
			userID,
			reservationID,
		); err != nil {
			return fmt.Errorf("release storage quota: %w", err)
		}

		return nil
	}

	if err := service.objectStore.Upload(
		ctx,
		objectKey,
		reader,
		sizeBytes,
		contentType,
	); err != nil {
		releaseErr := releaseReservation()

		if releaseErr != nil {
			return nil, errors.Join(
				fmt.Errorf("upload file content: %w", err),
				releaseErr,
			)
		}

		return nil, fmt.Errorf("upload file content: %w", err)
	}

	file, err := service.RegisterUploadedFile(
		ctx,
		ownerID,
		name,
		parentID,
		objectKey,
		sizeBytes,
		contentType,
	)
	if err != nil {
		cleanupErr := service.objectStore.Delete(ctx, objectKey)
		releaseErr := releaseReservation()

		return nil, errors.Join(
			err,
			cleanupErr,
			releaseErr,
		)
	}

	if reservationID != "" {
		if err := service.quotaManager.Commit(
			ctx,
			userID,
			reservationID,
		); err != nil {
			metadataCleanupErr := service.repository.DeleteFileAndVersions(
				ctx,
				file.ID,
				ownerID,
			)

			objectCleanupErr := service.objectStore.Delete(ctx, objectKey)
			releaseErr := releaseReservation()

			return nil, errors.Join(
				fmt.Errorf("commit storage quota: %w", err),
				metadataCleanupErr,
				objectCleanupErr,
				releaseErr,
			)
		}
	}

	return file, nil
}
