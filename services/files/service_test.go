package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type fakeFolderRepository struct {
	folders      map[bson.ObjectID]*Folder
	files        map[bson.ObjectID]*File
	fileVersions map[bson.ObjectID]*FileVersion
	versionError error
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

func newFakeFolderRepository() *fakeFolderRepository {
	return &fakeFolderRepository{
		folders:      make(map[bson.ObjectID]*Folder),
		files:        make(map[bson.ObjectID]*File),
		fileVersions: make(map[bson.ObjectID]*FileVersion),
	}
}

type fakeObjectStorage struct {
	objects     map[string]string
	uploadError error
	deleteError error
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

type fakeQuotaManager struct {
	reservationID string
	reserveError  error
	commitError   error
	releaseError  error

	reservedBytes int64
	reserveCalls  int
	commitCalls   int
	releaseCalls  int
}

func newFakeQuotaManager() *fakeQuotaManager {
	return &fakeQuotaManager{
		reservationID: "test-reservation",
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
	return manager.releaseError
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

func (manager *fakeQuotaManager) GetReservationStatus(
	ctx context.Context,
	userID string,
	reservationID string,
) (string, error) {
	return "reserved", nil
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
