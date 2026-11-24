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
	ErrFileNotFound      = errors.New("file not found")
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

	FindFiles(
		ctx context.Context,
		ownerID bson.ObjectID,
		parentID *bson.ObjectID,
	) ([]File, error)

	RenameFolder(
		ctx context.Context,
		folderID bson.ObjectID,
		ownerID bson.ObjectID,
		name string,
		updatedAt time.Time,
	) error

	RenameFile(
		ctx context.Context,
		fileID bson.ObjectID,
		ownerID bson.ObjectID,
		name string,
		updatedAt time.Time,
	) error

	CreateFile(ctx context.Context, file *File) error

	CreateFileVersion(ctx context.Context, version *FileVersion) error

	FindFileByID(
		ctx context.Context,
		fileID bson.ObjectID,
		ownerID bson.ObjectID,
	) (*File, error)

	FindLatestFileVersion(
		ctx context.Context,
		fileID bson.ObjectID,
	) (*FileVersion, error)

	UpdateFileVersion(
		ctx context.Context,
		fileID bson.ObjectID,
		ownerID bson.ObjectID,
		expectedVersion int,
		newVersion int,
		updatedAt time.Time,
	) error

	DeleteFileVersion(
		ctx context.Context,
		fileID bson.ObjectID,
		versionNumber int,
	) error

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

type FolderContents struct {
	Folders []Folder `json:"folders"`
	Files   []File   `json:"files"`
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

func (service *Service) ListFolderContents(
	ctx context.Context,
	ownerID bson.ObjectID,
	parentID *bson.ObjectID,
) (*FolderContents, error) {
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

	folders, err := service.repository.FindFolders(
		ctx,
		ownerID,
		parentID,
	)
	if err != nil {
		return nil, err
	}

	files, err := service.repository.FindFiles(
		ctx,
		ownerID,
		parentID,
	)
	if err != nil {
		return nil, err
	}

	return &FolderContents{
		Folders: folders,
		Files:   files,
	}, nil
}

func (service *Service) RenameFolder(
	ctx context.Context,
	ownerID bson.ObjectID,
	folderID bson.ObjectID,
	name string,
) error {
	name = strings.TrimSpace(name)

	if ownerID.IsZero() || folderID.IsZero() {
		return ErrParentNotFound
	}

	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, `/\`) {
		return ErrInvalidFolderName
	}

	_, err := service.repository.FindFolderByID(
		ctx,
		folderID,
		ownerID,
	)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrParentNotFound
		}

		return err
	}

	return service.repository.RenameFolder(
		ctx,
		folderID,
		ownerID,
		name,
		time.Now().UTC(),
	)
}

func (service *Service) RenameFile(
	ctx context.Context,
	ownerID bson.ObjectID,
	fileID bson.ObjectID,
	name string,
) error {
	name = strings.TrimSpace(name)

	if ownerID.IsZero() || fileID.IsZero() {
		return ErrFileNotFound
	}

	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, `/\`) {
		return ErrInvalidFileName
	}

	_, err := service.repository.FindFileByID(
		ctx,
		fileID,
		ownerID,
	)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrFileNotFound
		}

		return err
	}

	return service.repository.RenameFile(
		ctx,
		fileID,
		ownerID,
		name,
		time.Now().UTC(),
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
	contentType = strings.TrimSpace(contentType)

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

	if contentType == "" {
		return nil, errors.New("invalid file content type")
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

	name = strings.TrimSpace(name)
	objectKey = strings.TrimSpace(objectKey)

	if ownerID.IsZero() {
		return nil, errors.New("invalid file owner")
	}

	if name == "" {
		return nil, ErrInvalidFileName
	}

	if objectKey == "" {
		return nil, ErrInvalidObjectKey
	}

	if reader == nil {
		return nil, errors.New("file content reader is nil")
	}

	if sizeBytes < 0 {
		return nil, ErrInvalidFileSize
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

		if reservation == "" {
			return nil, errors.New("quota service returned an empty reservation ID")
		}

		reservationID = reservation
	}

	cleanupCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		10*time.Second,
	)
	defer cancel()

	releaseReservation := func() error {
		if reservationID == "" {
			return nil
		}

		if err := service.quotaManager.Release(
			cleanupCtx,
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

		return nil, errors.Join(
			fmt.Errorf("upload file content: %w", err),
			releaseErr,
		)
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
		cleanupErr := service.objectStore.Delete(
			cleanupCtx,
			objectKey,
		)
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
			commitErr := fmt.Errorf("commit storage quota: %w", err)

			status, statusErr := service.quotaManager.GetReservationStatus(
				cleanupCtx,
				userID,
				reservationID,
			)

			if statusErr != nil {
				return nil, errors.Join(
					commitErr,
					fmt.Errorf("check quota reservation status: %w", statusErr),
				)
			}

			switch status {
			case "committed":
				return file, nil

			case "reserved":
				metadataCleanupErr := service.repository.DeleteFileAndVersions(
					cleanupCtx,
					file.ID,
					ownerID,
				)

				objectCleanupErr := service.objectStore.Delete(
					cleanupCtx,
					objectKey,
				)

				releaseErr := releaseReservation()

				return nil, errors.Join(
					commitErr,
					metadataCleanupErr,
					objectCleanupErr,
					releaseErr,
				)

			case "released":
				metadataCleanupErr := service.repository.DeleteFileAndVersions(
					cleanupCtx,
					file.ID,
					ownerID,
				)

				objectCleanupErr := service.objectStore.Delete(
					cleanupCtx,
					objectKey,
				)

				return nil, errors.Join(
					commitErr,
					metadataCleanupErr,
					objectCleanupErr,
				)

			default:
				return nil, fmt.Errorf(
					"%w: unexpected reservation status %q",
					commitErr,
					status,
				)
			}
		}
	}

	return file, nil
}

func (service *Service) UploadFileVersion(
	ctx context.Context,
	ownerID bson.ObjectID,
	fileID bson.ObjectID,
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

	objectKey = strings.TrimSpace(objectKey)
	contentType = strings.TrimSpace(contentType)

	if ownerID.IsZero() {
		return nil, errors.New("invalid file owner")
	}

	if fileID.IsZero() {
		return nil, ErrFileNotFound
	}

	if objectKey == "" {
		return nil, ErrInvalidObjectKey
	}

	if reader == nil {
		return nil, errors.New("file content reader is nil")
	}

	if sizeBytes < 0 {
		return nil, ErrInvalidFileSize
	}

	if contentType == "" {
		return nil, errors.New("invalid file content type")
	}

	file, err := service.repository.FindFileByID(
		ctx,
		fileID,
		ownerID,
	)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrFileNotFound
		}

		return nil, fmt.Errorf("find file: %w", err)
	}

	latestVersion, err := service.repository.FindLatestFileVersion(
		ctx,
		fileID,
	)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("file has no registered versions")
		}

		return nil, fmt.Errorf("find latest file version: %w", err)
	}

	if latestVersion.VersionNumber != file.CurrentVersion {
		return nil, errors.New("file version metadata is inconsistent")
	}

	newVersionNumber := file.CurrentVersion + 1
	now := time.Now().UTC()
	oldUpdatedAt := file.UpdatedAt

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

		if reservation == "" {
			return nil, errors.New("quota service returned an empty reservation ID")
		}

		reservationID = reservation
	}

	cleanupCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		10*time.Second,
	)
	defer cancel()

	releaseReservation := func() error {
		if reservationID == "" {
			return nil
		}

		if err := service.quotaManager.Release(
			cleanupCtx,
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
		return nil, errors.Join(
			fmt.Errorf("upload file version content: %w", err),
			releaseReservation(),
		)
	}

	version := &FileVersion{
		ID:            bson.NewObjectID(),
		FileID:        fileID,
		VersionNumber: newVersionNumber,
		ObjectKey:     objectKey,
		SizeBytes:     sizeBytes,
		ContentType:   contentType,
		CreatedAt:     now,
	}

	if err := service.repository.CreateFileVersion(ctx, version); err != nil {
		return nil, errors.Join(
			fmt.Errorf("create file version: %w", err),
			service.objectStore.Delete(cleanupCtx, objectKey),
			releaseReservation(),
		)
	}

	if err := service.repository.UpdateFileVersion(
		ctx,
		fileID,
		ownerID,
		file.CurrentVersion,
		newVersionNumber,
		now,
	); err != nil {
		versionCleanupErr := service.repository.DeleteFileVersion(
			cleanupCtx,
			fileID,
			newVersionNumber,
		)

		var objectCleanupErr error
		if versionCleanupErr == nil {
			objectCleanupErr = service.objectStore.Delete(
				cleanupCtx,
				objectKey,
			)
		}

		return nil, errors.Join(
			fmt.Errorf("update current file version: %w", err),
			versionCleanupErr,
			objectCleanupErr,
			releaseReservation(),
		)
	}

	file.CurrentVersion = newVersionNumber
	file.UpdatedAt = now

	rollback := func() error {
		if err := service.repository.UpdateFileVersion(
			cleanupCtx,
			fileID,
			ownerID,
			newVersionNumber,
			newVersionNumber-1,
			oldUpdatedAt,
		); err != nil {
			return fmt.Errorf("restore previous file metadata: %w", err)
		}

		if err := service.repository.DeleteFileVersion(
			cleanupCtx,
			fileID,
			newVersionNumber,
		); err != nil {
			return fmt.Errorf("delete new version metadata: %w", err)
		}

		if err := service.objectStore.Delete(
			cleanupCtx,
			objectKey,
		); err != nil {
			return fmt.Errorf("delete new version content: %w", err)
		}

		return nil
	}

	if reservationID != "" {
		if err := service.quotaManager.Commit(
			ctx,
			userID,
			reservationID,
		); err != nil {
			commitErr := fmt.Errorf("commit storage quota: %w", err)

			status, statusErr := service.quotaManager.GetReservationStatus(
				cleanupCtx,
				userID,
				reservationID,
			)

			if statusErr != nil {
				return nil, errors.Join(
					commitErr,
					fmt.Errorf("check quota reservation status: %w", statusErr),
				)
			}

			switch status {
			case "committed":
				return file, nil

			case "reserved":
				rollbackErr := rollback()
				if rollbackErr != nil {
					return nil, errors.Join(commitErr, rollbackErr)
				}

				return nil, errors.Join(
					commitErr,
					releaseReservation(),
				)

			case "released":
				return nil, errors.Join(
					commitErr,
					rollback(),
				)

			default:
				return nil, fmt.Errorf(
					"%w: unexpected reservation status %q",
					commitErr,
					status,
				)
			}
		}
	}

	return file, nil
}
