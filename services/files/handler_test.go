package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/acilerts/cloud-drive/internal/platform"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func createTestHandler(t *testing.T) (*Handler, string) {
	t.Helper()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}

	userID := bson.NewObjectID()

	token, err := platform.SignToken(
		privateKey,
		userID.Hex(),
		"user",
		time.Hour,
	)
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}

	return NewHandler(nil, publicKey), token
}

func TestCreateFolderWithoutAuthorization(t *testing.T) {
	handler := NewHandler(nil, nil)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/folders",
		strings.NewReader(`{"name":"Dokumenti"}`),
	)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d",
			http.StatusUnauthorized, response.Code)
	}
}

func TestCreateFolderWithInvalidAuthorizationScheme(t *testing.T) {
	handler := NewHandler(nil, nil)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/folders",
		strings.NewReader(`{"name":"Dokumenti"}`),
	)

	request.Header.Set("Authorization", "Basic some-token")

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d",
			http.StatusUnauthorized, response.Code)
	}
}

func TestCreateFolderWithInvalidJSON(t *testing.T) {
	handler, token := createTestHandler(t)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/folders",
		strings.NewReader(`{"name":`),
	)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d",
			http.StatusBadRequest, response.Code)
	}
}

func TestCreateFolderWithInvalidParentID(t *testing.T) {
	handler, token := createTestHandler(t)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/folders",
		strings.NewReader(`{"name":"Dokumenti","parentId":"nije-validan-id"}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d",
			http.StatusBadRequest, response.Code)
	}
}

func TestCreateFolderWithUnknownOwnerIDField(t *testing.T) {
	handler, token := createTestHandler(t)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/folders",
		strings.NewReader(`{"name":"Dokumenti","ownerId":"123456789012345678901234"}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d",
			http.StatusBadRequest, response.Code)
	}
}

type handlerTestRepository struct {
	folders map[bson.ObjectID]*Folder
}

func newHandlerTestRepository() *handlerTestRepository {
	return &handlerTestRepository{
		folders: make(map[bson.ObjectID]*Folder),
	}
}

func (repository *handlerTestRepository) CreateFolder(
	ctx context.Context,
	folder *Folder,
) error {
	repository.folders[folder.ID] = folder
	return nil
}

func (repository *handlerTestRepository) FindFolderByID(
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

func (repository *handlerTestRepository) FindFolders(
	ctx context.Context,
	ownerID bson.ObjectID,
	parentID *bson.ObjectID,
) ([]Folder, error) {
	var folders []Folder

	for _, folder := range repository.folders {
		if folder.OwnerID != ownerID {
			continue
		}

		if parentID == nil {
			if folder.ParentID != nil {
				continue
			}
		} else {
			if folder.ParentID == nil || *folder.ParentID != *parentID {
				continue
			}
		}

		folders = append(folders, *folder)
	}

	return folders, nil
}

func TestCreateFolderSuccessfully(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}

	userID := bson.NewObjectID()

	token, err := platform.SignToken(
		privateKey,
		userID.Hex(),
		"user",
		time.Hour,
	)
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}

	repository := newHandlerTestRepository()
	service := NewService(repository)
	handler := NewHandler(service, publicKey)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/folders",
		strings.NewReader(`{"name":"Dokumenti"}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusCreated,
			response.Code,
			response.Body.String(),
		)
	}

	var folder Folder

	if err := json.NewDecoder(response.Body).Decode(&folder); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if folder.ID.IsZero() {
		t.Error("expected folder ID to be generated")
	}

	if folder.OwnerID != userID {
		t.Error("folder owner does not match authenticated user")
	}

	if folder.Name != "Dokumenti" {
		t.Errorf("expected folder name Dokumenti, got %s", folder.Name)
	}

	if folder.ParentID != nil {
		t.Error("expected root folder to have no parent")
	}

	if _, exists := repository.folders[folder.ID]; !exists {
		t.Error("folder was not saved in repository")
	}
}

func TestCreateNestedFoldersThroughHTTP(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}

	userID := bson.NewObjectID()

	token, err := platform.SignToken(
		privateKey,
		userID.Hex(),
		"user",
		time.Hour,
	)
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}

	repository := newHandlerTestRepository()
	service := NewService(repository)
	handler := NewHandler(service, publicKey)

	mux := http.NewServeMux()
	handler.Routes(mux)

	createFolder := func(name string, parentID *bson.ObjectID) Folder {
		t.Helper()

		body := map[string]string{
			"name": name,
		}

		if parentID != nil {
			body["parentId"] = parentID.Hex()
		}

		requestBody, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}

		request := httptest.NewRequest(
			http.MethodPost,
			"/folders",
			strings.NewReader(string(requestBody)),
		)
		request.Header.Set("Authorization", "Bearer "+token)

		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)

		if response.Code != http.StatusCreated {
			t.Fatalf(
				"creating folder %s: expected status %d, got %d: %s",
				name,
				http.StatusCreated,
				response.Code,
				response.Body.String(),
			)
		}

		var folder Folder

		if err := json.NewDecoder(response.Body).Decode(&folder); err != nil {
			t.Fatalf("decode folder response: %v", err)
		}

		return folder
	}

	root := createFolder("Dokumenti", nil)
	child := createFolder("Fakultet", &root.ID)
	grandchild := createFolder("Projekti", &child.ID)

	if root.ParentID != nil {
		t.Error("root folder should not have a parent")
	}

	if child.ParentID == nil || *child.ParentID != root.ID {
		t.Error("child folder should reference root folder")
	}

	if grandchild.ParentID == nil || *grandchild.ParentID != child.ID {
		t.Error("grandchild folder should reference child folder")
	}

	if root.OwnerID != userID ||
		child.OwnerID != userID ||
		grandchild.OwnerID != userID {
		t.Error("all folders should belong to the authenticated user")
	}

	if len(repository.folders) != 3 {
		t.Errorf(
			"expected 3 folders in repository, got %d",
			len(repository.folders),
		)
	}
}

func TestCreateFolderWithAdminRole(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}

	adminID := bson.NewObjectID()

	token, err := platform.SignToken(
		privateKey,
		adminID.Hex(),
		"admin",
		time.Hour,
	)
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}

	handler := NewHandler(nil, publicKey)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/folders",
		strings.NewReader(`{"name":"Dokumenti"}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			response.Code,
		)
	}
}

func TestListRootFoldersSuccessfully(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}

	userID := bson.NewObjectID()

	token, err := platform.SignToken(
		privateKey,
		userID.Hex(),
		"user",
		time.Hour,
	)
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}

	repository := newHandlerTestRepository()
	service := NewService(repository)

	_, err = service.CreateFolder(
		context.Background(), userID, "Dokumenti", nil,
	)
	if err != nil {
		t.Fatalf("create root folder: %v", err)
	}

	_, err = service.CreateFolder(
		context.Background(), userID, "Slike", nil,
	)
	if err != nil {
		t.Fatalf("create second root folder: %v", err)
	}

	handler := NewHandler(service, publicKey)
	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(http.MethodGet, "/folders", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s",
			http.StatusOK, response.Code, response.Body.String())
	}

	var folders []Folder
	if err := json.NewDecoder(response.Body).Decode(&folders); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(folders) != 2 {
		t.Fatalf("expected 2 root folders, got %d", len(folders))
	}
}

func TestListChildFoldersSuccessfully(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}

	userID := bson.NewObjectID()

	token, err := platform.SignToken(
		privateKey,
		userID.Hex(),
		"user",
		time.Hour,
	)
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}

	repository := newHandlerTestRepository()
	service := NewService(repository)

	parent, err := service.CreateFolder(
		context.Background(), userID, "Dokumenti", nil,
	)
	if err != nil {
		t.Fatalf("create parent folder: %v", err)
	}

	_, err = service.CreateFolder(
		context.Background(), userID, "Fakultet", &parent.ID,
	)
	if err != nil {
		t.Fatalf("create child folder: %v", err)
	}

	handler := NewHandler(service, publicKey)
	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(
		http.MethodGet,
		"/folders?parentId="+parent.ID.Hex(),
		nil,
	)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s",
			http.StatusOK, response.Code, response.Body.String())
	}

	var folders []Folder
	if err := json.NewDecoder(response.Body).Decode(&folders); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(folders) != 1 {
		t.Fatalf("expected 1 child folder, got %d", len(folders))
	}

	if folders[0].Name != "Fakultet" {
		t.Errorf("expected folder Fakultet, got %s", folders[0].Name)
	}
}

func TestListFoldersWithoutAuthorization(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}

	repository := newHandlerTestRepository()
	service := NewService(repository)
	handler := NewHandler(service, publicKey)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(http.MethodGet, "/folders", nil)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d",
			http.StatusUnauthorized, response.Code)
	}
}

func TestListFoldersWithInvalidParentID(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}

	userID := bson.NewObjectID()

	token, err := platform.SignToken(
		privateKey,
		userID.Hex(),
		"user",
		time.Hour,
	)
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}

	repository := newHandlerTestRepository()
	service := NewService(repository)
	handler := NewHandler(service, publicKey)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(
		http.MethodGet,
		"/folders?parentId=neispravan-id",
		nil,
	)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d",
			http.StatusBadRequest, response.Code)
	}
}
