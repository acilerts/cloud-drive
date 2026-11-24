package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type fakeFolderRepository struct {
	folders            map[bson.ObjectID]*Folder
	files              map[bson.ObjectID]*File
	fileVersions       map[bson.ObjectID]*FileVersion
	versionError       error
	updateVersionError error
}

func (repository *fakeFolderRepository) FindFolders(
	ctx context.Context,
	ownerID bson.ObjectID,
	parentID *bson.ObjectID,
) ([]Folder, error) {
	var folders []Folder

	for _, folder := range repository.folders {
		if folder.OwnerID != ownerID {
			continue
		}

		if parentID == nil && folder.ParentID != nil {
			continue
		}

		if parentID != nil {
			if folder.ParentID == nil || *folder.ParentID != *parentID {
				continue
			}
		}

		folders = append(folders, *folder)
	}

	return folders, nil
}

func (repository *fakeFolderRepository) FindFiles(
	ctx context.Context,
	ownerID bson.ObjectID,
	parentID *bson.ObjectID,
) ([]File, error) {
	var files []File

	for _, file := range repository.files {
		if file.OwnerID != ownerID {
			continue
		}

		if parentID == nil && file.ParentID != nil {
			continue
		}

		if parentID != nil {
			if file.ParentID == nil || *file.ParentID != *parentID {
				continue
			}
		}

		files = append(files, *file)
	}

	return files, nil
}

func (repository *fakeFolderRepository) RenameFolder(
	ctx context.Context,
	folderID bson.ObjectID,
	ownerID bson.ObjectID,
	name string,
	updatedAt time.Time,
) error {
	folder, exists := repository.folders[folderID]
	if !exists || folder.OwnerID != ownerID {
		return mongo.ErrNoDocuments
	}

	folder.Name = name
	folder.UpdatedAt = updatedAt

	return nil
}

func (repository *fakeFolderRepository) RenameFile(
	ctx context.Context,
	fileID bson.ObjectID,
	ownerID bson.ObjectID,
	name string,
	updatedAt time.Time,
) error {
	file, exists := repository.files[fileID]
	if !exists || file.OwnerID != ownerID {
		return mongo.ErrNoDocuments
	}

	file.Name = name
	file.UpdatedAt = updatedAt

	return nil
}

func newFakeFolderRepository() *fakeFolderRepository {
	return &fakeFolderRepository{
		folders:      make(map[bson.ObjectID]*Folder),
		files:        make(map[bson.ObjectID]*File),
		fileVersions: make(map[bson.ObjectID]*FileVersion),
	}
}

type fakeObjectStorage struct {
	objects          map[string]string
	uploadError      error
	deleteError      error
	uploadContextErr error
}

func newFakeObjectStorage() *fakeObjectStorage {
	return &fakeObjectStorage{
		objects: make(map[string]string),
	}
}

func (storage *fakeObjectStorage) Upload(
	ctx context.Context,
	objectKey string,
	reader io.Reader,
	size int64,
	contentType string,
) error {
	if storage.uploadError != nil {
		return storage.uploadError
	}

	if storage.uploadContextErr != nil {
		return storage.uploadContextErr
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	if int64(len(content)) != size {
		return errors.New("uploaded content size mismatch")
	}

	storage.objects[objectKey] = string(content)
	return nil
}

func (storage *fakeObjectStorage) Delete(
	ctx context.Context,
	objectKey string,
) error {
	if storage.deleteError != nil {
		return storage.deleteError
	}

	delete(storage.objects, objectKey)
	return nil
}

func (repository *fakeFolderRepository) CreateFolder(
	ctx context.Context,
	folder *Folder,
) error {
	repository.folders[folder.ID] = folder
	return nil
}

func (repository *fakeFolderRepository) FindFolderByID(
	ctx context.Context,
	folderID bson.ObjectID,
	ownerID bson.ObjectID,
) (*Folder, error) {
	folder, exists := repository.folders[folderID]

	if !exists || folder.OwnerID != ownerID {
		return nil, mongo.ErrNoDocuments
	}

	return folder, nil
}

func (repository *fakeFolderRepository) CreateFile(
	ctx context.Context,
	file *File,
) error {
	repository.files[file.ID] = file
	return nil
}

func (repository *fakeFolderRepository) CreateFileVersion(
	ctx context.Context,
	version *FileVersion,
) error {
	if repository.versionError != nil {
		return repository.versionError
	}

	repository.fileVersions[version.ID] = version
	return nil
}

func (repository *fakeFolderRepository) DeleteFile(
	ctx context.Context,
	fileID bson.ObjectID,
	ownerID bson.ObjectID,
) error {
	file, exists := repository.files[fileID]
	if !exists || file.OwnerID != ownerID {
		return nil
	}

	delete(repository.files, fileID)
	return nil
}

func (repository *fakeFolderRepository) DeleteFileAndVersions(
	ctx context.Context,
	fileID bson.ObjectID,
	ownerID bson.ObjectID,
) error {
	file, exists := repository.files[fileID]
	if !exists || file.OwnerID != ownerID {
		return nil
	}

	for versionID, version := range repository.fileVersions {
		if version.FileID == fileID {
			delete(repository.fileVersions, versionID)
		}
	}

	delete(repository.files, fileID)

	return nil
}

func (repository *fakeFolderRepository) FindFileByID(
	ctx context.Context,
	fileID bson.ObjectID,
	ownerID bson.ObjectID,
) (*File, error) {
	file, exists := repository.files[fileID]
	if !exists || file.OwnerID != ownerID {
		return nil, mongo.ErrNoDocuments
	}

	return file, nil
}

func (repository *fakeFolderRepository) FindLatestFileVersion(
	ctx context.Context,
	fileID bson.ObjectID,
) (*FileVersion, error) {
	var latest *FileVersion

	for _, version := range repository.fileVersions {
		if version.FileID != fileID {
			continue
		}

		if latest == nil || version.VersionNumber > latest.VersionNumber {
			latest = version
		}
	}

	if latest == nil {
		return nil, mongo.ErrNoDocuments
	}

	return latest, nil
}

func (repository *fakeFolderRepository) UpdateFileVersion(
	ctx context.Context,
	fileID bson.ObjectID,
	ownerID bson.ObjectID,
	expectedVersion int,
	newVersion int,
	updatedAt time.Time,
) error {
	if repository.updateVersionError != nil {
		return repository.updateVersionError
	}
	file, exists := repository.files[fileID]
	if !exists || file.OwnerID != ownerID {
		return mongo.ErrNoDocuments
	}

	if file.CurrentVersion != expectedVersion {
		return errors.New("file version changed concurrently")
	}

	file.CurrentVersion = newVersion
	file.UpdatedAt = updatedAt

	return nil
}

func (repository *fakeFolderRepository) DeleteFileVersion(
	ctx context.Context,
	fileID bson.ObjectID,
	versionNumber int,
) error {
	for versionID, version := range repository.fileVersions {
		if version.FileID == fileID && version.VersionNumber == versionNumber {
			delete(repository.fileVersions, versionID)
			return nil
		}
	}

	return nil
}

type fakeQuotaManager struct {
	reservationID     string
	reserveError      error
	commitError       error
	releaseError      error
	releaseContextErr error
	statusError       error
	reservationStatus string

	reservedBytes int64
	reserveCalls  int
	commitCalls   int
	releaseCalls  int
	statusCalls   int
}

func newFakeQuotaManager() *fakeQuotaManager {
	return &fakeQuotaManager{
		reservationID:     "test-reservation",
		reservationStatus: "reserved",
	}
}

func (manager *fakeQuotaManager) Reserve(
	ctx context.Context,
	userID string,
	bytes int64,
) (string, error) {
	manager.reserveCalls++
	manager.reservedBytes = bytes

	if manager.reserveError != nil {
		return "", manager.reserveError
	}

	return manager.reservationID, nil
}

func (manager *fakeQuotaManager) Commit(
	ctx context.Context,
	userID string,
	reservationID string,
) error {
	manager.commitCalls++
	return manager.commitError
}

func (manager *fakeQuotaManager) Release(
	ctx context.Context,
	userID string,
	reservationID string,
) error {
	manager.releaseCalls++
	manager.releaseContextErr = ctx.Err()
	return manager.releaseError
}

func (manager *fakeQuotaManager) GetReservationStatus(
	ctx context.Context,
	userID string,
	reservationID string,
) (string, error) {
	manager.statusCalls++

	if manager.statusError != nil {
		return "", manager.statusError
	}

	return manager.reservationStatus, nil
}

func TestCreateRootFolder(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	ownerID := bson.NewObjectID()

	folder, err := service.CreateFolder(
		context.Background(),
		ownerID,
		"Dokumenti",
		nil,
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if folder.Name != "Dokumenti" {
		t.Errorf("expected folder name Dokumenti, got %s", folder.Name)
	}

	if folder.OwnerID != ownerID {
		t.Error("folder owner does not match")
	}

	if folder.ParentID != nil {
		t.Error("root folder should not have a parent")
	}

	if folder.ID.IsZero() {
		t.Error("folder ID should not be empty")
	}

	if folder.CreatedAt.IsZero() || folder.UpdatedAt.IsZero() {
		t.Error("folder timestamps should not be empty")
	}
}

func TestCreateNestedFolders(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	ownerID := bson.NewObjectID()
	ctx := context.Background()

	root, err := service.CreateFolder(
		ctx,
		ownerID,
		"Dokumenti",
		nil,
	)
	if err != nil {
		t.Fatalf("create root folder: %v", err)
	}

	child, err := service.CreateFolder(
		ctx,
		ownerID,
		"Fakultet",
		&root.ID,
	)
	if err != nil {
		t.Fatalf("create child folder: %v", err)
	}

	grandchild, err := service.CreateFolder(
		ctx,
		ownerID,
		"Projekti",
		&child.ID,
	)
	if err != nil {
		t.Fatalf("create grandchild folder: %v", err)
	}

	if child.ParentID == nil || *child.ParentID != root.ID {
		t.Error("child folder has incorrect parent")
	}

	if grandchild.ParentID == nil || *grandchild.ParentID != child.ID {
		t.Error("grandchild folder has incorrect parent")
	}
}

func TestCreateFolderWithEmptyName(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	_, err := service.CreateFolder(
		context.Background(),
		bson.NewObjectID(),
		"   ",
		nil,
	)

	if !errors.Is(err, ErrInvalidFolderName) {
		t.Errorf("expected ErrInvalidFolderName, got %v", err)
	}
}

func TestCreateFolderWithInvalidOwner(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	_, err := service.CreateFolder(
		context.Background(),
		bson.ObjectID{},
		"Dokumenti",
		nil,
	)

	if err == nil {
		t.Error("expected error for invalid owner")
	}
}

func TestCreateFolderWithNonexistentParent(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	ownerID := bson.NewObjectID()
	parentID := bson.NewObjectID()

	_, err := service.CreateFolder(
		context.Background(),
		ownerID,
		"Fakultet",
		&parentID,
	)

	if !errors.Is(err, ErrParentNotFound) {
		t.Errorf("expected ErrParentNotFound, got %v", err)
	}
}

func TestCreateFolderWithAnotherUsersParent(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	owner1 := bson.NewObjectID()
	owner2 := bson.NewObjectID()

	parent, err := service.CreateFolder(
		context.Background(),
		owner1,
		"Privatni folder",
		nil,
	)
	if err != nil {
		t.Fatalf("create parent folder: %v", err)
	}

	_, err = service.CreateFolder(
		context.Background(),
		owner2,
		"Pokusaj pristupa",
		&parent.ID,
	)

	if !errors.Is(err, ErrParentNotFound) {
		t.Errorf("expected ErrParentNotFound, got %v", err)
	}
}

func TestListRootFolders(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	ownerID := bson.NewObjectID()

	root1, err := service.CreateFolder(
		context.Background(), ownerID, "Dokumenti", nil,
	)
	if err != nil {
		t.Fatalf("create root folder: %v", err)
	}

	_, err = service.CreateFolder(
		context.Background(), ownerID, "Slike", nil,
	)
	if err != nil {
		t.Fatalf("create second root folder: %v", err)
	}

	_, err = service.CreateFolder(
		context.Background(), ownerID, "Fakultet", &root1.ID,
	)
	if err != nil {
		t.Fatalf("create child folder: %v", err)
	}

	folders, err := service.ListFolders(
		context.Background(), ownerID, nil,
	)
	if err != nil {
		t.Fatalf("list root folders: %v", err)
	}

	if len(folders) != 2 {
		t.Fatalf("expected 2 root folders, got %d", len(folders))
	}

	for _, folder := range folders {
		if folder.ParentID != nil {
			t.Errorf("expected root folder, got parentId %v", folder.ParentID)
		}
	}
}

func TestListChildFolders(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	ownerID := bson.NewObjectID()

	parent, err := service.CreateFolder(
		context.Background(), ownerID, "Dokumenti", nil,
	)
	if err != nil {
		t.Fatalf("create parent folder: %v", err)
	}

	_, err = service.CreateFolder(
		context.Background(), ownerID, "Fakultet", &parent.ID,
	)
	if err != nil {
		t.Fatalf("create first child: %v", err)
	}

	_, err = service.CreateFolder(
		context.Background(), ownerID, "Posao", &parent.ID,
	)
	if err != nil {
		t.Fatalf("create second child: %v", err)
	}

	folders, err := service.ListFolders(
		context.Background(), ownerID, &parent.ID,
	)
	if err != nil {
		t.Fatalf("list child folders: %v", err)
	}

	if len(folders) != 2 {
		t.Fatalf("expected 2 child folders, got %d", len(folders))
	}

	for _, folder := range folders {
		if folder.ParentID == nil || *folder.ParentID != parent.ID {
			t.Errorf("folder %q has incorrect parent", folder.Name)
		}
	}
}

func TestListFoldersWithAnotherUsersParent(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	ownerID := bson.NewObjectID()
	anotherOwnerID := bson.NewObjectID()

	parent, err := service.CreateFolder(
		context.Background(), anotherOwnerID, "Privatno", nil,
	)
	if err != nil {
		t.Fatalf("create another user's folder: %v", err)
	}

	_, err = service.ListFolders(
		context.Background(), ownerID, &parent.ID,
	)

	if !errors.Is(err, ErrParentNotFound) {
		t.Fatalf("expected ErrParentNotFound, got %v", err)
	}
}

func TestListFoldersWithInvalidOwner(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	_, err := service.ListFolders(
		context.Background(), bson.NilObjectID, nil,
	)

	if err == nil {
		t.Fatal("expected error for invalid owner")
	}
}

func TestRegisterUploadedFile(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	ownerID := bson.NewObjectID()

	file, err := service.RegisterUploadedFile(
		context.Background(),
		ownerID,
		"dokument.pdf",
		nil,
		"uploads/test/dokument.pdf",
		1024,
		"application/pdf",
	)

	if err != nil {
		t.Fatalf("register uploaded file: %v", err)
	}

	if file.Name != "dokument.pdf" {
		t.Errorf("unexpected file name: %s", file.Name)
	}

	if file.CurrentVersion != 1 {
		t.Errorf("expected current version 1, got %d", file.CurrentVersion)
	}

	if file.OwnerID != ownerID {
		t.Error("file owner does not match")
	}

	if _, exists := repository.files[file.ID]; !exists {
		t.Error("file was not saved in repository")
	}

	if len(repository.fileVersions) != 1 {
		t.Fatalf("expected 1 file version, got %d", len(repository.fileVersions))
	}

	for _, version := range repository.fileVersions {
		if version.FileID != file.ID {
			t.Error("file version references incorrect file")
		}

		if version.VersionNumber != 1 {
			t.Errorf("expected version number 1, got %d", version.VersionNumber)
		}

		if version.ObjectKey != "uploads/test/dokument.pdf" {
			t.Errorf("unexpected object key: %s", version.ObjectKey)
		}

		if version.SizeBytes != 1024 {
			t.Errorf("expected size 1024, got %d", version.SizeBytes)
		}

		if version.ContentType != "application/pdf" {
			t.Errorf("unexpected content type: %s", version.ContentType)
		}
	}
}

func TestRegisterUploadedFileTrimsWhitespace(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	file, err := service.RegisterUploadedFile(
		context.Background(),
		bson.NewObjectID(),
		"  dokument.pdf  ",
		nil,
		"  uploads/test/dokument.pdf  ",
		1024,
		"application/pdf",
	)

	if err != nil {
		t.Fatalf("register uploaded file: %v", err)
	}

	if file.Name != "dokument.pdf" {
		t.Errorf("expected trimmed file name, got %q", file.Name)
	}

	for _, version := range repository.fileVersions {
		if version.ObjectKey != "uploads/test/dokument.pdf" {
			t.Errorf("expected trimmed object key, got %q", version.ObjectKey)
		}
	}
}

func TestRegisterUploadedFileRejectsEmptyContentType(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	_, err := service.RegisterUploadedFile(
		context.Background(),
		bson.NewObjectID(),
		"dokument.pdf",
		nil,
		"uploads/test/dokument.pdf",
		1024,
		"",
	)

	if err == nil {
		t.Fatal("expected error for empty content type")
	}

	if len(repository.files) != 0 {
		t.Fatal("file should not be created")
	}

	if len(repository.fileVersions) != 0 {
		t.Fatal("file version should not be created")
	}
}

func TestRegisterUploadedFileRejectsInvalidSizeAndObjectKey(t *testing.T) {
	tests := []struct {
		name      string
		objectKey string
		sizeBytes int64
	}{
		{
			name:      "negative file size",
			objectKey: "uploads/test/file.pdf",
			sizeBytes: -1,
		},
		{
			name:      "empty object key",
			objectKey: "",
			sizeBytes: 1024,
		},
		{
			name:      "whitespace object key",
			objectKey: "   ",
			sizeBytes: 1024,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newFakeFolderRepository()
			service := NewService(repository)

			_, err := service.RegisterUploadedFile(
				context.Background(),
				bson.NewObjectID(),
				"dokument.pdf",
				nil,
				test.objectKey,
				test.sizeBytes,
				"application/pdf",
			)

			if err == nil {
				t.Fatal("expected validation error")
			}

			if len(repository.files) != 0 {
				t.Error("file should not be created")
			}

			if len(repository.fileVersions) != 0 {
				t.Error("file version should not be created")
			}
		})
	}
}

func TestRegisterUploadedFileCleansUpOnVersionError(t *testing.T) {
	repository := newFakeFolderRepository()
	repository.versionError = errors.New("simulated version insert failure")

	service := NewService(repository)

	_, err := service.RegisterUploadedFile(
		context.Background(),
		bson.NewObjectID(),
		"dokument.pdf",
		nil,
		"uploads/test/dokument.pdf",
		1024,
		"application/pdf",
	)

	if err == nil {
		t.Fatal("expected error when creating file version")
	}

	if len(repository.files) != 0 {
		t.Errorf("expected file cleanup, got %d files", len(repository.files))
	}

	if len(repository.fileVersions) != 0 {
		t.Errorf("expected no file versions, got %d", len(repository.fileVersions))
	}
}

func TestUploadFileFailsWhenObjectStorageIsNotConfigured(t *testing.T) {
	repository := newFakeFolderRepository()
	quotaManager := newFakeQuotaManager()

	service := NewServiceWithDependencies(repository, nil, quotaManager)

	_, err := service.UploadFile(
		context.Background(),
		bson.NewObjectID(),
		"dokument.txt",
		nil,
		"uploads/test/dokument.txt",
		strings.NewReader("test"),
		4,
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected error when object storage is not configured")
	}

	if err.Error() != "object storage is not configured" {
		t.Errorf("unexpected error: %v", err)
	}

	if quotaManager.reserveCalls != 0 {
		t.Error("quota should not be reserved")
	}
}

func TestUploadFileFailsWhenQuotaManagerIsNotConfigured(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()

	service := NewServiceWithDependencies(repository, storage, nil)

	_, err := service.UploadFile(
		context.Background(),
		bson.NewObjectID(),
		"dokument.txt",
		nil,
		"uploads/test/dokument.txt",
		strings.NewReader("test"),
		4,
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected error when quota manager is not configured")
	}

	if err.Error() != "quota manager is not configured" {
		t.Errorf("unexpected error: %v", err)
	}

	if len(storage.objects) != 0 {
		t.Error("file should not be uploaded")
	}
}

func TestUploadFileSuccessfully(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	service := NewServiceWithDependencies(repository, storage, quotaManager)

	ownerID := bson.NewObjectID()
	content := "Sadrzaj test dokumenta"
	objectKey := "uploads/test/dokument.txt"

	file, err := service.UploadFile(
		context.Background(),
		ownerID,
		"dokument.txt",
		nil,
		objectKey,
		strings.NewReader(content),
		int64(len(content)),
		"text/plain",
	)
	if err != nil {
		t.Fatalf("upload file: %v", err)
	}

	if file.Name != "dokument.txt" {
		t.Errorf("unexpected file name: %s", file.Name)
	}

	if file.OwnerID != ownerID {
		t.Error("file owner does not match")
	}

	if len(repository.files) != 1 {
		t.Errorf("expected 1 file, got %d", len(repository.files))
	}

	if len(repository.fileVersions) != 1 {
		t.Errorf("expected 1 file version, got %d", len(repository.fileVersions))
	}

	if storage.objects[objectKey] != content {
		t.Error("uploaded content does not match")
	}

	if quotaManager.reserveCalls != 1 {
		t.Errorf("expected 1 quota reservation, got %d", quotaManager.reserveCalls)
	}

	if quotaManager.reservedBytes != int64(len(content)) {
		t.Errorf("expected %d reserved bytes, got %d", len(content), quotaManager.reservedBytes)
	}

	if quotaManager.commitCalls != 1 {
		t.Errorf("expected 1 quota commit, got %d", quotaManager.commitCalls)
	}

	if quotaManager.releaseCalls != 0 {
		t.Errorf("expected 0 quota releases, got %d", quotaManager.releaseCalls)
	}
}

func TestUploadFileSuccessfullyWithZeroBytes(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	service := NewServiceWithDependencies(repository, storage, quotaManager)

	ownerID := bson.NewObjectID()
	objectKey := "uploads/test/empty.txt"

	file, err := service.UploadFile(
		context.Background(),
		ownerID,
		"empty.txt",
		nil,
		objectKey,
		strings.NewReader(""),
		0,
		"text/plain",
	)
	if err != nil {
		t.Fatalf("upload empty file: %v", err)
	}

	if file == nil {
		t.Fatal("expected file metadata")
	}

	if len(repository.files) != 1 {
		t.Errorf("expected 1 file, got %d", len(repository.files))
	}

	if len(repository.fileVersions) != 1 {
		t.Errorf("expected 1 file version, got %d", len(repository.fileVersions))
	}

	if _, exists := storage.objects[objectKey]; !exists {
		t.Error("empty file was not uploaded to object storage")
	}

	if quotaManager.reserveCalls != 0 {
		t.Errorf("expected 0 quota reservations, got %d", quotaManager.reserveCalls)
	}

	if quotaManager.commitCalls != 0 {
		t.Errorf("expected 0 quota commits, got %d", quotaManager.commitCalls)
	}

	if quotaManager.releaseCalls != 0 {
		t.Errorf("expected 0 quota releases, got %d", quotaManager.releaseCalls)
	}
}

func TestUploadFileFailsWhenQuotaReservationFails(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	quotaManager.reserveError = errors.New("insufficient storage quota")

	service := NewServiceWithDependencies(repository, storage, quotaManager)

	ownerID := bson.NewObjectID()
	content := "Sadrzaj test dokumenta"
	objectKey := "uploads/test/dokument.txt"

	_, err := service.UploadFile(
		context.Background(),
		ownerID,
		"dokument.txt",
		nil,
		objectKey,
		strings.NewReader(content),
		int64(len(content)),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected quota reservation error")
	}

	if quotaManager.reserveCalls != 1 {
		t.Errorf("expected 1 quota reservation, got %d", quotaManager.reserveCalls)
	}

	if quotaManager.commitCalls != 0 {
		t.Errorf("expected 0 quota commits, got %d", quotaManager.commitCalls)
	}

	if quotaManager.releaseCalls != 0 {
		t.Errorf("expected 0 quota releases, got %d", quotaManager.releaseCalls)
	}

	if len(storage.objects) != 0 {
		t.Errorf("expected no uploaded objects, got %d", len(storage.objects))
	}

	if len(repository.files) != 0 {
		t.Errorf("expected no files in repository, got %d", len(repository.files))
	}

	if len(repository.fileVersions) != 0 {
		t.Errorf("expected no file versions, got %d", len(repository.fileVersions))
	}
}

func TestUploadFileReleasesQuotaWhenObjectStorageFails(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	storage.uploadError = errors.New("object storage unavailable")

	service := NewServiceWithDependencies(repository, storage, quotaManager)

	ownerID := bson.NewObjectID()
	content := "Sadrzaj test dokumenta"
	objectKey := "uploads/test/dokument.txt"

	_, err := service.UploadFile(
		context.Background(),
		ownerID,
		"dokument.txt",
		nil,
		objectKey,
		strings.NewReader(content),
		int64(len(content)),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected object storage error")
	}

	if quotaManager.reserveCalls != 1 {
		t.Errorf("expected 1 quota reservation, got %d", quotaManager.reserveCalls)
	}

	if quotaManager.releaseCalls != 1 {
		t.Errorf("expected 1 quota release, got %d", quotaManager.releaseCalls)
	}

	if quotaManager.commitCalls != 0 {
		t.Errorf("expected 0 quota commits, got %d", quotaManager.commitCalls)
	}

	if len(repository.files) != 0 {
		t.Errorf("expected no files in repository, got %d", len(repository.files))
	}

	if len(repository.fileVersions) != 0 {
		t.Errorf("expected no file versions, got %d", len(repository.fileVersions))
	}
}

func TestUploadFileReleasesQuotaWhenContextIsCanceled(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	storage.uploadContextErr = context.Canceled

	service := NewServiceWithDependencies(repository, storage, quotaManager)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ownerID := bson.NewObjectID()
	content := "Sadrzaj test dokumenta"

	_, err := service.UploadFile(
		ctx,
		ownerID,
		"dokument.txt",
		nil,
		"uploads/test/dokument.txt",
		strings.NewReader(content),
		int64(len(content)),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected context cancellation error")
	}

	if quotaManager.reserveCalls != 1 {
		t.Errorf("expected 1 quota reservation, got %d", quotaManager.reserveCalls)
	}

	if quotaManager.releaseCalls != 1 {
		t.Errorf("expected 1 quota release, got %d", quotaManager.releaseCalls)
	}

	if quotaManager.commitCalls != 0 {
		t.Errorf("expected 0 quota commits, got %d", quotaManager.commitCalls)
	}

	if len(repository.files) != 0 {
		t.Errorf("expected no files, got %d", len(repository.files))
	}

	if len(storage.objects) != 0 {
		t.Errorf("expected no objects, got %d", len(storage.objects))
	}

	if quotaManager.releaseContextErr != nil {
		t.Errorf(
			"expected active context during quota release, got %v",
			quotaManager.releaseContextErr,
		)
	}
}

func TestUploadFileCleansUpObjectOnMetadataError(t *testing.T) {
	repository := newFakeFolderRepository()
	repository.versionError = errors.New("simulated version insert failure")

	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	service := NewServiceWithDependencies(repository, storage, quotaManager)

	ownerID := bson.NewObjectID()
	content := "Sadrzaj test dokumenta"
	objectKey := "uploads/test/dokument.txt"

	_, err := service.UploadFile(
		context.Background(),
		ownerID,
		"dokument.txt",
		nil,
		objectKey,
		strings.NewReader(content),
		int64(len(content)),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected upload error")
	}

	if len(storage.objects) != 0 {
		t.Errorf(
			"expected uploaded object to be deleted, got %d objects",
			len(storage.objects),
		)
	}

	if len(repository.files) != 0 {
		t.Errorf("expected no file metadata, got %d files", len(repository.files))
	}

	if quotaManager.reserveCalls != 1 {
		t.Errorf("expected 1 quota reservation, got %d", quotaManager.reserveCalls)
	}

	if quotaManager.releaseCalls != 1 {
		t.Errorf("expected 1 quota release, got %d", quotaManager.releaseCalls)
	}

	if quotaManager.commitCalls != 0 {
		t.Errorf("expected 0 quota commits, got %d", quotaManager.commitCalls)
	}
}

func TestUploadFileReturnsReleaseErrorWhenObjectStorageFails(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	storage.uploadError = errors.New("object storage unavailable")
	quotaManager.releaseError = errors.New("quota release unavailable")

	service := NewServiceWithDependencies(repository, storage, quotaManager)

	ownerID := bson.NewObjectID()
	content := "Sadrzaj test dokumenta"

	_, err := service.UploadFile(
		context.Background(),
		ownerID,
		"dokument.txt",
		nil,
		"uploads/test/dokument.txt",
		strings.NewReader(content),
		int64(len(content)),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected upload and quota release errors")
	}

	if !strings.Contains(err.Error(), "quota release unavailable") {
		t.Errorf("expected quota release error, got: %v", err)
	}

	if quotaManager.releaseCalls != 1 {
		t.Errorf("expected 1 quota release, got %d", quotaManager.releaseCalls)
	}

	if quotaManager.commitCalls != 0 {
		t.Errorf("expected 0 quota commits, got %d", quotaManager.commitCalls)
	}
}

func TestUploadFileKeepsFileWhenCommitSucceededDespiteError(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	quotaManager.commitError = errors.New("commit response lost")
	quotaManager.reservationStatus = "committed"

	service := NewServiceWithDependencies(repository, storage, quotaManager)

	ownerID := bson.NewObjectID()
	content := "Sadrzaj test dokumenta"
	objectKey := "uploads/test/dokument.txt"

	file, err := service.UploadFile(
		context.Background(),
		ownerID,
		"dokument.txt",
		nil,
		objectKey,
		strings.NewReader(content),
		int64(len(content)),
		"text/plain",
	)

	if err != nil {
		t.Fatalf("expected successful upload, got: %v", err)
	}

	if file == nil {
		t.Fatal("expected file to be returned")
	}

	if repository.files[file.ID] == nil {
		t.Error("file metadata should remain stored")
	}

	if storage.objects[objectKey] != content {
		t.Error("uploaded object should remain stored")
	}

	if quotaManager.statusCalls != 1 {
		t.Errorf("expected 1 status check, got %d", quotaManager.statusCalls)
	}

	if quotaManager.releaseCalls != 0 {
		t.Errorf("expected no quota release, got %d", quotaManager.releaseCalls)
	}
}

func TestUploadFileCleansUpWhenCommitFailedAndReservationIsReserved(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	quotaManager.commitError = errors.New("commit unavailable")
	quotaManager.reservationStatus = "reserved"

	service := NewServiceWithDependencies(repository, storage, quotaManager)

	ownerID := bson.NewObjectID()
	content := "Sadrzaj test dokumenta"
	objectKey := "uploads/test/dokument.txt"

	_, err := service.UploadFile(
		context.Background(),
		ownerID,
		"dokument.txt",
		nil,
		objectKey,
		strings.NewReader(content),
		int64(len(content)),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected commit error")
	}

	if len(repository.files) != 0 {
		t.Error("file metadata should be deleted")
	}

	if _, exists := storage.objects[objectKey]; exists {
		t.Error("uploaded object should be deleted")
	}

	if quotaManager.statusCalls != 1 {
		t.Errorf("expected 1 status check, got %d", quotaManager.statusCalls)
	}

	if quotaManager.releaseCalls != 1 {
		t.Errorf("expected 1 quota release, got %d", quotaManager.releaseCalls)
	}
}

func TestUploadFileKeepsDataWhenReservationStatusCheckFails(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	quotaManager.commitError = errors.New("commit unavailable")
	quotaManager.statusError = errors.New("quota service unavailable")

	service := NewServiceWithDependencies(repository, storage, quotaManager)

	ownerID := bson.NewObjectID()
	content := "Sadrzaj test dokumenta"
	objectKey := "uploads/test/dokument.txt"

	_, err := service.UploadFile(
		context.Background(),
		ownerID,
		"dokument.txt",
		nil,
		objectKey,
		strings.NewReader(content),
		int64(len(content)),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected commit and status check errors")
	}

	if len(repository.files) != 1 {
		t.Errorf("expected file metadata to remain, got %d files", len(repository.files))
	}

	if storage.objects[objectKey] != content {
		t.Error("uploaded object should remain stored")
	}

	if quotaManager.statusCalls != 1 {
		t.Errorf("expected 1 status check, got %d", quotaManager.statusCalls)
	}

	if quotaManager.releaseCalls != 0 {
		t.Errorf("expected no quota release, got %d", quotaManager.releaseCalls)
	}
}

func TestUploadFileCleansUpWhenReservationIsAlreadyReleased(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	quotaManager.commitError = errors.New("commit unavailable")
	quotaManager.reservationStatus = "released"

	service := NewServiceWithDependencies(repository, storage, quotaManager)

	ownerID := bson.NewObjectID()
	content := "Sadrzaj test dokumenta"
	objectKey := "uploads/test/dokument.txt"

	_, err := service.UploadFile(
		context.Background(),
		ownerID,
		"dokument.txt",
		nil,
		objectKey,
		strings.NewReader(content),
		int64(len(content)),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected commit error")
	}

	if len(repository.files) != 0 {
		t.Error("file metadata should be deleted")
	}

	if _, exists := storage.objects[objectKey]; exists {
		t.Error("uploaded object should be deleted")
	}

	if quotaManager.statusCalls != 1 {
		t.Errorf("expected 1 status check, got %d", quotaManager.statusCalls)
	}

	if quotaManager.releaseCalls != 0 {
		t.Errorf("expected no additional quota release, got %d", quotaManager.releaseCalls)
	}
}

func TestUploadFileKeepsDataWhenReservationStatusIsUnexpected(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	quotaManager.commitError = errors.New("commit failed")
	quotaManager.reservationStatus = "unknown"

	service := NewServiceWithDependencies(repository, storage, quotaManager)

	ownerID := bson.NewObjectID()
	content := "test file content"

	file, err := service.UploadFile(
		context.Background(),
		ownerID,
		"test.txt",
		nil,
		"uploads/test.txt",
		strings.NewReader(content),
		int64(len(content)),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected error for unexpected reservation status")
	}

	if file != nil {
		t.Fatal("expected nil file")
	}

	if len(repository.files) != 1 {
		t.Errorf("expected file metadata to be preserved, got %d files", len(repository.files))
	}

	if len(storage.objects) != 1 {
		t.Errorf("expected uploaded object to be preserved, got %d objects", len(storage.objects))
	}

	if quotaManager.statusCalls != 1 {
		t.Errorf("expected 1 status check, got %d", quotaManager.statusCalls)
	}

	if quotaManager.releaseCalls != 0 {
		t.Errorf("expected no quota release, got %d calls", quotaManager.releaseCalls)
	}
}

func TestUploadFileVersionSuccessfully(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	service := NewServiceWithDependencies(
		repository,
		storage,
		quotaManager,
	)

	ownerID := bson.NewObjectID()
	fileID := bson.NewObjectID()

	oldObjectKey := "uploads/test/document-v1.txt"
	newObjectKey := "uploads/test/document-v2.txt"

	oldContent := "old file content"
	newContent := "updated file content"

	oldUpdatedAt := time.Now().UTC().Add(-time.Hour)

	file := &File{
		ID:             fileID,
		OwnerID:        ownerID,
		Name:           "document.txt",
		CurrentVersion: 1,
		CreatedAt:      oldUpdatedAt,
		UpdatedAt:      oldUpdatedAt,
	}

	repository.files[fileID] = file

	firstVersion := &FileVersion{
		ID:            bson.NewObjectID(),
		FileID:        fileID,
		VersionNumber: 1,
		ObjectKey:     oldObjectKey,
		SizeBytes:     int64(len(oldContent)),
		ContentType:   "text/plain",
		CreatedAt:     oldUpdatedAt,
	}

	repository.fileVersions[firstVersion.ID] = firstVersion
	storage.objects[oldObjectKey] = oldContent

	updatedFile, err := service.UploadFileVersion(
		context.Background(),
		ownerID,
		fileID,
		newObjectKey,
		strings.NewReader(newContent),
		int64(len(newContent)),
		"text/plain",
	)

	if err != nil {
		t.Fatalf("upload new file version: %v", err)
	}

	if updatedFile.CurrentVersion != 2 {
		t.Errorf(
			"expected current version 2, got %d",
			updatedFile.CurrentVersion,
		)
	}

	if updatedFile.UpdatedAt.Equal(oldUpdatedAt) {
		t.Error("file UpdatedAt should be updated")
	}

	if updatedFile.Name != "document.txt" {
		t.Errorf("file name changed unexpectedly: %s", updatedFile.Name)
	}

	if len(repository.fileVersions) != 2 {
		t.Fatalf(
			"expected 2 file versions, got %d",
			len(repository.fileVersions),
		)
	}

	if storage.objects[oldObjectKey] != oldContent {
		t.Error("previous file version content was changed")
	}

	if storage.objects[newObjectKey] != newContent {
		t.Error("new file version content was not uploaded correctly")
	}

	var secondVersion *FileVersion

	for _, version := range repository.fileVersions {
		if version.VersionNumber == 2 {
			secondVersion = version
			break
		}
	}

	if secondVersion == nil {
		t.Fatal("second file version was not created")
	}

	if secondVersion.ObjectKey != newObjectKey {
		t.Errorf(
			"expected object key %q, got %q",
			newObjectKey,
			secondVersion.ObjectKey,
		)
	}

	if quotaManager.reserveCalls != 1 {
		t.Errorf("expected 1 quota reservation, got %d", quotaManager.reserveCalls)
	}

	if quotaManager.reservedBytes != int64(len(newContent)) {
		t.Errorf(
			"expected %d reserved bytes, got %d",
			len(newContent),
			quotaManager.reservedBytes,
		)
	}

	if quotaManager.commitCalls != 1 {
		t.Errorf("expected 1 quota commit, got %d", quotaManager.commitCalls)
	}
}

func TestUploadFileVersionRejectsFileNotOwnedByUser(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	service := NewServiceWithDependencies(
		repository,
		storage,
		quotaManager,
	)

	ownerID := bson.NewObjectID()
	anotherUserID := bson.NewObjectID()
	fileID := bson.NewObjectID()

	repository.files[fileID] = &File{
		ID:             fileID,
		OwnerID:        anotherUserID,
		Name:           "private.txt",
		CurrentVersion: 1,
	}

	_, err := service.UploadFileVersion(
		context.Background(),
		ownerID,
		fileID,
		"uploads/new-version.txt",
		strings.NewReader("new content"),
		int64(len("new content")),
		"text/plain",
	)

	if !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("expected ErrFileNotFound, got %v", err)
	}

	if quotaManager.reserveCalls != 0 {
		t.Errorf("expected no quota reservation, got %d", quotaManager.reserveCalls)
	}

	if len(storage.objects) != 0 {
		t.Errorf("expected no uploaded objects, got %d", len(storage.objects))
	}
}

func TestUploadFileVersionRejectsQuotaReservation(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	quotaManager.reserveError = errors.New("quota exceeded")

	service := NewServiceWithDependencies(
		repository,
		storage,
		quotaManager,
	)

	ownerID := bson.NewObjectID()
	fileID := bson.NewObjectID()
	now := time.Now().UTC()

	file := &File{
		ID:             fileID,
		OwnerID:        ownerID,
		Name:           "document.txt",
		CurrentVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	repository.files[fileID] = file

	version := &FileVersion{
		ID:            bson.NewObjectID(),
		FileID:        fileID,
		VersionNumber: 1,
		ObjectKey:     "uploads/document-v1.txt",
		SizeBytes:     10,
		ContentType:   "text/plain",
		CreatedAt:     now,
	}

	repository.fileVersions[version.ID] = version

	_, err := service.UploadFileVersion(
		context.Background(),
		ownerID,
		fileID,
		"uploads/document-v2.txt",
		strings.NewReader("new content"),
		int64(len("new content")),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected quota reservation error")
	}

	if quotaManager.reserveCalls != 1 {
		t.Errorf("expected 1 reservation call, got %d", quotaManager.reserveCalls)
	}

	if quotaManager.commitCalls != 0 {
		t.Errorf("expected no commit calls, got %d", quotaManager.commitCalls)
	}

	if file.CurrentVersion != 1 {
		t.Errorf("expected current version 1, got %d", file.CurrentVersion)
	}

	if len(repository.fileVersions) != 1 {
		t.Errorf("expected 1 version, got %d", len(repository.fileVersions))
	}

	if len(storage.objects) != 0 {
		t.Errorf("expected no uploaded objects, got %d", len(storage.objects))
	}
}

func TestUploadFileVersionReleasesQuotaOnUploadError(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	storage.uploadError = errors.New("minio upload failed")

	service := NewServiceWithDependencies(
		repository,
		storage,
		quotaManager,
	)

	ownerID := bson.NewObjectID()
	fileID := bson.NewObjectID()
	now := time.Now().UTC()

	file := &File{
		ID:             fileID,
		OwnerID:        ownerID,
		Name:           "document.txt",
		CurrentVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	repository.files[fileID] = file

	version := &FileVersion{
		ID:            bson.NewObjectID(),
		FileID:        fileID,
		VersionNumber: 1,
		ObjectKey:     "uploads/document-v1.txt",
		SizeBytes:     10,
		ContentType:   "text/plain",
		CreatedAt:     now,
	}

	repository.fileVersions[version.ID] = version

	_, err := service.UploadFileVersion(
		context.Background(),
		ownerID,
		fileID,
		"uploads/document-v2.txt",
		strings.NewReader("new content"),
		int64(len("new content")),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected upload error")
	}

	if quotaManager.reserveCalls != 1 {
		t.Errorf("expected 1 reservation call, got %d", quotaManager.reserveCalls)
	}

	if quotaManager.releaseCalls != 1 {
		t.Errorf("expected 1 release call, got %d", quotaManager.releaseCalls)
	}

	if quotaManager.commitCalls != 0 {
		t.Errorf("expected no commit calls, got %d", quotaManager.commitCalls)
	}

	if file.CurrentVersion != 1 {
		t.Errorf("expected current version 1, got %d", file.CurrentVersion)
	}

	if len(repository.fileVersions) != 1 {
		t.Errorf("expected 1 version, got %d", len(repository.fileVersions))
	}
}

func TestUploadFileVersionCleansUpOnMetadataError(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	repository.versionError = errors.New("database insert failed")

	service := NewServiceWithDependencies(
		repository,
		storage,
		quotaManager,
	)

	ownerID := bson.NewObjectID()
	fileID := bson.NewObjectID()
	now := time.Now().UTC()

	file := &File{
		ID:             fileID,
		OwnerID:        ownerID,
		Name:           "document.txt",
		CurrentVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	repository.files[fileID] = file

	version := &FileVersion{
		ID:            bson.NewObjectID(),
		FileID:        fileID,
		VersionNumber: 1,
		ObjectKey:     "uploads/document-v1.txt",
		SizeBytes:     10,
		ContentType:   "text/plain",
		CreatedAt:     now,
	}

	repository.fileVersions[version.ID] = version
	storage.objects[version.ObjectKey] = "old content"

	_, err := service.UploadFileVersion(
		context.Background(),
		ownerID,
		fileID,
		"uploads/document-v2.txt",
		strings.NewReader("new content"),
		int64(len("new content")),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected metadata creation error")
	}

	if quotaManager.reserveCalls != 1 {
		t.Errorf("expected 1 reservation call, got %d", quotaManager.reserveCalls)
	}

	if quotaManager.releaseCalls != 1 {
		t.Errorf("expected 1 release call, got %d", quotaManager.releaseCalls)
	}

	if quotaManager.commitCalls != 0 {
		t.Errorf("expected no commit calls, got %d", quotaManager.commitCalls)
	}

	if file.CurrentVersion != 1 {
		t.Errorf("expected current version 1, got %d", file.CurrentVersion)
	}

	if len(repository.fileVersions) != 1 {
		t.Errorf("expected 1 version, got %d", len(repository.fileVersions))
	}

	if storage.objects[version.ObjectKey] != "old content" {
		t.Error("previous version content was changed")
	}

	if len(storage.objects) != 1 {
		t.Errorf("expected only previous object to remain, got %d", len(storage.objects))
	}
}

func TestUploadFileVersionCleansUpOnCurrentVersionUpdateError(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	service := NewServiceWithDependencies(
		repository,
		storage,
		quotaManager,
	)

	ownerID := bson.NewObjectID()
	fileID := bson.NewObjectID()
	now := time.Now().UTC()

	file := &File{
		ID:             fileID,
		OwnerID:        ownerID,
		Name:           "document.txt",
		CurrentVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	repository.files[fileID] = file

	version := &FileVersion{
		ID:            bson.NewObjectID(),
		FileID:        fileID,
		VersionNumber: 1,
		ObjectKey:     "uploads/document-v1.txt",
		SizeBytes:     10,
		ContentType:   "text/plain",
		CreatedAt:     now,
	}

	repository.fileVersions[version.ID] = version
	repository.updateVersionError = errors.New("database update failed")

	_, err := service.UploadFileVersion(
		context.Background(),
		ownerID,
		fileID,
		"uploads/document-v3.txt",
		strings.NewReader("new content"),
		int64(len("new content")),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected current version update error")
	}

	if quotaManager.reserveCalls != 1 {
		t.Errorf("expected 1 reservation call, got %d", quotaManager.reserveCalls)
	}

	if quotaManager.releaseCalls != 1 {
		t.Errorf("expected 1 release call, got %d", quotaManager.releaseCalls)
	}

	if quotaManager.commitCalls != 0 {
		t.Errorf("expected no commit calls, got %d", quotaManager.commitCalls)
	}

	if len(repository.fileVersions) != 1 {
		t.Errorf("expected 1 version after cleanup, got %d", len(repository.fileVersions))
	}

	if len(storage.objects) != 0 {
		t.Errorf("expected no uploaded objects, got %d", len(storage.objects))
	}

	if file.CurrentVersion != 1 {
		t.Errorf("expected current version 1 to remain, got %d", file.CurrentVersion)
	}
}

func TestUploadFileVersionCommitErrorButReservationCommitted(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	quotaManager.commitError = errors.New("commit response lost")
	quotaManager.reservationStatus = "committed"

	service := NewServiceWithDependencies(
		repository,
		storage,
		quotaManager,
	)

	ownerID := bson.NewObjectID()
	fileID := bson.NewObjectID()
	now := time.Now().UTC()

	file := &File{
		ID:             fileID,
		OwnerID:        ownerID,
		Name:           "document.txt",
		CurrentVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	repository.files[fileID] = file

	version := &FileVersion{
		ID:            bson.NewObjectID(),
		FileID:        fileID,
		VersionNumber: 1,
		ObjectKey:     "uploads/document-v1.txt",
		SizeBytes:     10,
		ContentType:   "text/plain",
		CreatedAt:     now,
	}

	repository.fileVersions[version.ID] = version

	updatedFile, err := service.UploadFileVersion(
		context.Background(),
		ownerID,
		fileID,
		"uploads/document-v2.txt",
		strings.NewReader("new content"),
		int64(len("new content")),
		"text/plain",
	)

	if err != nil {
		t.Fatalf("expected successful upload after committed reservation, got %v", err)
	}

	if updatedFile.CurrentVersion != 2 {
		t.Errorf("expected current version 2, got %d", updatedFile.CurrentVersion)
	}

	if len(repository.fileVersions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(repository.fileVersions))
	}

	if len(storage.objects) != 1 {
		t.Errorf("expected 1 newly uploaded object, got %d", len(storage.objects))
	}

	if quotaManager.commitCalls != 1 {
		t.Errorf("expected 1 commit call, got %d", quotaManager.commitCalls)
	}

	if quotaManager.statusCalls != 1 {
		t.Errorf("expected 1 status check, got %d", quotaManager.statusCalls)
	}

	if quotaManager.releaseCalls != 0 {
		t.Errorf("expected no release calls, got %d", quotaManager.releaseCalls)
	}
}

func TestUploadFileVersionRollsBackWhenReservationStillReserved(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	quotaManager.commitError = errors.New("commit failed")
	quotaManager.reservationStatus = "reserved"

	service := NewServiceWithDependencies(
		repository,
		storage,
		quotaManager,
	)

	ownerID := bson.NewObjectID()
	fileID := bson.NewObjectID()
	now := time.Now().UTC()

	file := &File{
		ID:             fileID,
		OwnerID:        ownerID,
		Name:           "document.txt",
		CurrentVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	repository.files[fileID] = file

	version := &FileVersion{
		ID:            bson.NewObjectID(),
		FileID:        fileID,
		VersionNumber: 1,
		ObjectKey:     "uploads/document-v1.txt",
		SizeBytes:     10,
		ContentType:   "text/plain",
		CreatedAt:     now,
	}

	repository.fileVersions[version.ID] = version
	storage.objects[version.ObjectKey] = "old content"

	_, err := service.UploadFileVersion(
		context.Background(),
		ownerID,
		fileID,
		"uploads/document-v2.txt",
		strings.NewReader("new content"),
		int64(len("new content")),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected commit error")
	}

	if file.CurrentVersion != 1 {
		t.Errorf("expected current version 1 after rollback, got %d", file.CurrentVersion)
	}

	if !file.UpdatedAt.Equal(now) {
		t.Error("expected UpdatedAt to be restored")
	}

	if len(repository.fileVersions) != 1 {
		t.Errorf("expected 1 version after rollback, got %d", len(repository.fileVersions))
	}

	if storage.objects[version.ObjectKey] != "old content" {
		t.Error("previous version content was changed")
	}

	if len(storage.objects) != 1 {
		t.Errorf("expected only previous object to remain, got %d", len(storage.objects))
	}

	if quotaManager.releaseCalls != 1 {
		t.Errorf("expected 1 release call, got %d", quotaManager.releaseCalls)
	}

	if quotaManager.statusCalls != 1 {
		t.Errorf("expected 1 status check, got %d", quotaManager.statusCalls)
	}
}

func TestUploadFileVersionRollsBackWhenReservationAlreadyReleased(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	quotaManager.commitError = errors.New("commit failed")
	quotaManager.reservationStatus = "released"

	service := NewServiceWithDependencies(
		repository,
		storage,
		quotaManager,
	)

	ownerID := bson.NewObjectID()
	fileID := bson.NewObjectID()
	now := time.Now().UTC()

	file := &File{
		ID:             fileID,
		OwnerID:        ownerID,
		Name:           "document.txt",
		CurrentVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	repository.files[fileID] = file

	version := &FileVersion{
		ID:            bson.NewObjectID(),
		FileID:        fileID,
		VersionNumber: 1,
		ObjectKey:     "uploads/document-v1.txt",
		SizeBytes:     10,
		ContentType:   "text/plain",
		CreatedAt:     now,
	}

	repository.fileVersions[version.ID] = version
	storage.objects[version.ObjectKey] = "old content"

	_, err := service.UploadFileVersion(
		context.Background(),
		ownerID,
		fileID,
		"uploads/document-v2.txt",
		strings.NewReader("new content"),
		int64(len("new content")),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected commit error")
	}

	if file.CurrentVersion != 1 {
		t.Errorf("expected current version 1, got %d", file.CurrentVersion)
	}

	if !file.UpdatedAt.Equal(now) {
		t.Error("expected UpdatedAt to be restored")
	}

	if len(repository.fileVersions) != 1 {
		t.Errorf("expected 1 version after rollback, got %d", len(repository.fileVersions))
	}

	if storage.objects[version.ObjectKey] != "old content" {
		t.Error("previous version content was changed")
	}

	if len(storage.objects) != 1 {
		t.Errorf("expected only previous object to remain, got %d", len(storage.objects))
	}

	if quotaManager.releaseCalls != 0 {
		t.Errorf("expected no additional release calls, got %d", quotaManager.releaseCalls)
	}

	if quotaManager.statusCalls != 1 {
		t.Errorf("expected 1 status check, got %d", quotaManager.statusCalls)
	}
}

func TestUploadFileVersionPreservesDataWhenReservationStatusCheckFails(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	quotaManager.commitError = errors.New("commit response lost")
	quotaManager.statusError = errors.New("quota service unavailable")

	service := NewServiceWithDependencies(
		repository,
		storage,
		quotaManager,
	)

	ownerID := bson.NewObjectID()
	fileID := bson.NewObjectID()
	now := time.Now().UTC()

	file := &File{
		ID:             fileID,
		OwnerID:        ownerID,
		Name:           "document.txt",
		CurrentVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	repository.files[fileID] = file

	version := &FileVersion{
		ID:            bson.NewObjectID(),
		FileID:        fileID,
		VersionNumber: 1,
		ObjectKey:     "uploads/document-v1.txt",
		SizeBytes:     10,
		ContentType:   "text/plain",
		CreatedAt:     now,
	}

	repository.fileVersions[version.ID] = version
	storage.objects[version.ObjectKey] = "old content"

	newObjectKey := "uploads/document-v2.txt"

	_, err := service.UploadFileVersion(
		context.Background(),
		ownerID,
		fileID,
		newObjectKey,
		strings.NewReader("new content"),
		int64(len("new content")),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected reservation status check error")
	}

	if file.CurrentVersion != 2 {
		t.Errorf("expected current version 2 to be preserved, got %d", file.CurrentVersion)
	}

	if len(repository.fileVersions) != 2 {
		t.Errorf("expected 2 versions to be preserved, got %d", len(repository.fileVersions))
	}

	if storage.objects[version.ObjectKey] != "old content" {
		t.Error("previous version content was changed")
	}

	if storage.objects[newObjectKey] != "new content" {
		t.Error("new version content should be preserved")
	}

	if quotaManager.statusCalls != 1 {
		t.Errorf("expected 1 status check, got %d", quotaManager.statusCalls)
	}

	if quotaManager.releaseCalls != 0 {
		t.Errorf("expected no release calls, got %d", quotaManager.releaseCalls)
	}
}

func TestUploadFileVersionDoesNotReleaseQuotaWhenRollbackFails(t *testing.T) {
	repository := newFakeFolderRepository()
	storage := newFakeObjectStorage()
	quotaManager := newFakeQuotaManager()

	quotaManager.commitError = errors.New("commit failed")
	quotaManager.reservationStatus = "reserved"
	storage.deleteError = errors.New("object deletion failed")

	service := NewServiceWithDependencies(
		repository,
		storage,
		quotaManager,
	)

	ownerID := bson.NewObjectID()
	fileID := bson.NewObjectID()
	now := time.Now().UTC()

	file := &File{
		ID:             fileID,
		OwnerID:        ownerID,
		Name:           "document.txt",
		CurrentVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	repository.files[fileID] = file

	oldObjectKey := "uploads/document-v1.txt"
	newObjectKey := "uploads/document-v2.txt"

	version := &FileVersion{
		ID:            bson.NewObjectID(),
		FileID:        fileID,
		VersionNumber: 1,
		ObjectKey:     oldObjectKey,
		SizeBytes:     10,
		ContentType:   "text/plain",
		CreatedAt:     now,
	}

	repository.fileVersions[version.ID] = version
	storage.objects[oldObjectKey] = "old content"

	_, err := service.UploadFileVersion(
		context.Background(),
		ownerID,
		fileID,
		newObjectKey,
		strings.NewReader("new content"),
		int64(len("new content")),
		"text/plain",
	)

	if err == nil {
		t.Fatal("expected error when rollback fails")
	}

	if quotaManager.releaseCalls != 0 {
		t.Errorf(
			"expected no quota release when rollback fails, got %d",
			quotaManager.releaseCalls,
		)
	}

	if file.CurrentVersion != 1 {
		t.Errorf(
			"expected current version 1 after rollback, got %d",
			file.CurrentVersion,
		)
	}

	if len(repository.fileVersions) != 1 {
		t.Errorf(
			"expected 1 version after rollback, got %d",
			len(repository.fileVersions),
		)
	}

	if storage.objects[newObjectKey] != "new content" {
		t.Error("expected new object to remain because deletion failed")
	}
}

func TestListFolderContentsSuccessfully(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	ownerID := bson.NewObjectID()
	parentID := bson.NewObjectID()

	parentFolder := &Folder{
		ID:      parentID,
		OwnerID: ownerID,
		Name:    "Documents",
	}

	subfolder := &Folder{
		ID:       bson.NewObjectID(),
		OwnerID:  ownerID,
		Name:     "Projects",
		ParentID: &parentID,
	}

	file := &File{
		ID:       bson.NewObjectID(),
		OwnerID:  ownerID,
		Name:     "report.pdf",
		ParentID: &parentID,
	}

	repository.folders[parentID] = parentFolder
	repository.folders[subfolder.ID] = subfolder
	repository.files[file.ID] = file

	contents, err := service.ListFolderContents(
		context.Background(),
		ownerID,
		&parentID,
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(contents.Folders) != 1 {
		t.Fatalf(
			"expected 1 subfolder, got %d",
			len(contents.Folders),
		)
	}

	if contents.Folders[0].ID != subfolder.ID {
		t.Error("returned incorrect subfolder")
	}

	if len(contents.Files) != 1 {
		t.Fatalf(
			"expected 1 file, got %d",
			len(contents.Files),
		)
	}

	if contents.Files[0].ID != file.ID {
		t.Error("returned incorrect file")
	}
}

func TestListFolderContentsRejectsAnotherUsersFolder(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	ownerID := bson.NewObjectID()
	anotherOwnerID := bson.NewObjectID()
	parentID := bson.NewObjectID()

	repository.folders[parentID] = &Folder{
		ID:      parentID,
		OwnerID: anotherOwnerID,
		Name:    "Private",
	}

	contents, err := service.ListFolderContents(
		context.Background(),
		ownerID,
		&parentID,
	)

	if contents != nil {
		t.Error("expected nil contents for another user's folder")
	}

	if !errors.Is(err, ErrParentNotFound) {
		t.Errorf(
			"expected ErrParentNotFound, got %v",
			err,
		)
	}
}

func TestRenameFolderSuccessfully(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	ownerID := bson.NewObjectID()
	folderID := bson.NewObjectID()

	oldTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	folder := &Folder{
		ID:        folderID,
		OwnerID:   ownerID,
		Name:      "OldName",
		CreatedAt: oldTime,
		UpdatedAt: oldTime,
	}

	repository.folders[folderID] = folder

	err := service.RenameFolder(
		context.Background(),
		ownerID,
		folderID,
		"NewName",
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if folder.Name != "NewName" {
		t.Errorf("expected name NewName, got %s", folder.Name)
	}

	if !folder.UpdatedAt.After(oldTime) {
		t.Error("expected UpdatedAt to be updated")
	}

	if !folder.CreatedAt.Equal(oldTime) {
		t.Error("CreatedAt should remain unchanged")
	}
}

func TestRenameFolderRejectsAnotherUsersFolder(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	ownerID := bson.NewObjectID()
	anotherOwnerID := bson.NewObjectID()
	folderID := bson.NewObjectID()

	folder := &Folder{
		ID:      folderID,
		OwnerID: anotherOwnerID,
		Name:    "Private",
	}

	repository.folders[folderID] = folder

	err := service.RenameFolder(
		context.Background(),
		ownerID,
		folderID,
		"Renamed",
	)

	if !errors.Is(err, ErrParentNotFound) {
		t.Errorf("expected ErrParentNotFound, got %v", err)
	}

	if folder.Name != "Private" {
		t.Errorf(
			"expected original name Private, got %s",
			folder.Name,
		)
	}
}

func TestRenameFolderRejectsInvalidName(t *testing.T) {
	invalidNames := []string{
		"",
		"   ",
		".",
		"..",
		"folder/name",
		`folder\name`,
	}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			repository := newFakeFolderRepository()
			service := NewService(repository)

			ownerID := bson.NewObjectID()
			folderID := bson.NewObjectID()

			folder := &Folder{
				ID:      folderID,
				OwnerID: ownerID,
				Name:    "Original",
			}

			repository.folders[folderID] = folder

			err := service.RenameFolder(
				context.Background(),
				ownerID,
				folderID,
				name,
			)

			if !errors.Is(err, ErrInvalidFolderName) {
				t.Errorf(
					"expected ErrInvalidFolderName, got %v",
					err,
				)
			}

			if folder.Name != "Original" {
				t.Errorf(
					"expected original name to remain unchanged, got %s",
					folder.Name,
				)
			}
		})
	}
}

func TestRenameFileSuccessfully(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	ownerID := bson.NewObjectID()
	fileID := bson.NewObjectID()

	oldTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	file := &File{
		ID:             fileID,
		OwnerID:        ownerID,
		Name:           "old-report.pdf",
		CurrentVersion: 1,
		CreatedAt:      oldTime,
		UpdatedAt:      oldTime,
	}

	repository.files[fileID] = file

	err := service.RenameFile(
		context.Background(),
		ownerID,
		fileID,
		"new-report.pdf",
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if file.Name != "new-report.pdf" {
		t.Errorf("expected new-report.pdf, got %s", file.Name)
	}

	if !file.UpdatedAt.After(oldTime) {
		t.Error("expected UpdatedAt to be updated")
	}

	if !file.CreatedAt.Equal(oldTime) {
		t.Error("CreatedAt should remain unchanged")
	}

	if file.CurrentVersion != 1 {
		t.Error("renaming should not change the current file version")
	}
}

func TestRenameFileRejectsAnotherUsersFile(t *testing.T) {
	repository := newFakeFolderRepository()
	service := NewService(repository)

	ownerID := bson.NewObjectID()
	anotherOwnerID := bson.NewObjectID()
	fileID := bson.NewObjectID()

	file := &File{
		ID:      fileID,
		OwnerID: anotherOwnerID,
		Name:    "private.pdf",
	}

	repository.files[fileID] = file

	err := service.RenameFile(
		context.Background(),
		ownerID,
		fileID,
		"renamed.pdf",
	)

	if !errors.Is(err, ErrFileNotFound) {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}

	if file.Name != "private.pdf" {
		t.Errorf(
			"expected original name private.pdf, got %s",
			file.Name,
		)
	}
}

func TestRenameFileRejectsInvalidName(t *testing.T) {
	invalidNames := []string{
		"",
		"   ",
		".",
		"..",
		"file/name.pdf",
		`file\name.pdf`,
	}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			repository := newFakeFolderRepository()
			service := NewService(repository)

			ownerID := bson.NewObjectID()
			fileID := bson.NewObjectID()

			file := &File{
				ID:      fileID,
				OwnerID: ownerID,
				Name:    "original.pdf",
			}

			repository.files[fileID] = file

			err := service.RenameFile(
				context.Background(),
				ownerID,
				fileID,
				name,
			)

			if !errors.Is(err, ErrInvalidFileName) {
				t.Errorf(
					"expected ErrInvalidFileName, got %v",
					err,
				)
			}

			if file.Name != "original.pdf" {
				t.Errorf(
					"expected original name to remain unchanged, got %s",
					file.Name,
				)
			}
		})
	}
}
