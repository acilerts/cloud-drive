package main

import (
	"context"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type fakeFolderRepository struct {
	folders map[bson.ObjectID]*Folder
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
		folders: make(map[bson.ObjectID]*Folder),
	}
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
