package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"

	"bytes"
	"mime/multipart"
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
	folders      map[bson.ObjectID]*Folder
	files        map[bson.ObjectID]*File
	fileVersions map[bson.ObjectID]*FileVersion
}

func newHandlerTestRepository() *handlerTestRepository {
	return &handlerTestRepository{
		folders:      make(map[bson.ObjectID]*Folder),
		files:        make(map[bson.ObjectID]*File),
		fileVersions: make(map[bson.ObjectID]*FileVersion),
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

func (repository *handlerTestRepository) CreateFile(
	ctx context.Context,
	file *File,
) error {
	repository.files[file.ID] = file
	return nil
}

func (repository *handlerTestRepository) CreateFileVersion(
	ctx context.Context,
	version *FileVersion,
) error {
	repository.fileVersions[version.ID] = version
	return nil
}

func (repository *handlerTestRepository) DeleteFile(
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

func (repository *handlerTestRepository) DeleteFileAndVersions(
	ctx context.Context,
	fileID bson.ObjectID,
	ownerID bson.ObjectID,
) error {
	return repository.DeleteFile(ctx, fileID, ownerID)
}

func (repository *handlerTestRepository) FindFileByID(
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

func (repository *handlerTestRepository) FindLatestFileVersion(
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

func (repository *handlerTestRepository) UpdateFileVersion(
	ctx context.Context,
	fileID bson.ObjectID,
	ownerID bson.ObjectID,
	expectedVersion int,
	newVersion int,
	updatedAt time.Time,
) error {
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

func (repository *handlerTestRepository) DeleteFileVersion(
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

func newUploadRequest(
	t *testing.T,
	token string,
	filename string,
	content string,
	parentID string,
) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	filePart, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}

	if _, err := filePart.Write([]byte(content)); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}

	if parentID != "" {
		if err := writer.WriteField("parentId", parentID); err != nil {
			t.Fatalf("write parentId: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/files",
		&body,
	)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	return request
}

func TestUploadFileThroughHTTPSuccessfully(t *testing.T) {
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
	objectStore := newFakeObjectStorage()
	service := NewServiceWithDependencies(repository, objectStore, newFakeQuotaManager())
	handler := NewHandler(service, publicKey)

	mux := http.NewServeMux()
	handler.Routes(mux)

	content := "Sadrzaj test dokumenta"
	request := newUploadRequest(
		t,
		token,
		"dokument.txt",
		content,
		"",
	)

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

	var file File
	if err := json.NewDecoder(response.Body).Decode(&file); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if file.ID.IsZero() {
		t.Error("expected file ID to be generated")
	}

	if file.OwnerID != userID {
		t.Error("file owner does not match authenticated user")
	}

	if file.Name != "dokument.txt" {
		t.Errorf("expected filename dokument.txt, got %s", file.Name)
	}

	if file.CurrentVersion != 1 {
		t.Errorf("expected current version 1, got %d", file.CurrentVersion)
	}

	if len(repository.files) != 1 {
		t.Errorf("expected 1 file in repository, got %d", len(repository.files))
	}

	if len(repository.fileVersions) != 1 {
		t.Fatalf(
			"expected 1 file version in repository, got %d",
			len(repository.fileVersions),
		)
	}

	var storedVersion *FileVersion
	for _, version := range repository.fileVersions {
		storedVersion = version
	}

	if storedVersion.FileID != file.ID {
		t.Errorf("file version references incorrect file ID")
	}

	if storedVersion.VersionNumber != 1 {
		t.Errorf(
			"expected version number 1, got %d",
			storedVersion.VersionNumber,
		)
	}

	if storedVersion.SizeBytes != int64(len(content)) {
		t.Errorf(
			"expected size %d, got %d",
			len(content),
			storedVersion.SizeBytes,
		)
	}

	if storedVersion.ContentType != "text/plain; charset=utf-8" {
		t.Errorf(
			"expected content type text/plain; charset=utf-8, got %s",
			storedVersion.ContentType,
		)
	}

	if storedVersion.ObjectKey == "" {
		t.Error("expected object key to be generated")
	}

	if _, exists := objectStore.objects[storedVersion.ObjectKey]; !exists {
		t.Error("expected file version object key to exist in object storage")
	}

	if len(objectStore.objects) != 1 {
		t.Fatalf("expected 1 object in storage, got %d", len(objectStore.objects))
	}

	var storedContent string
	for _, value := range objectStore.objects {
		storedContent = value
	}

	if storedContent != content {
		t.Errorf("stored content does not match uploaded content")
	}
}

func TestUploadFileTooLarge(t *testing.T) {
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
	objectStore := newFakeObjectStorage()
	service := NewServiceWithDependencies(
		repository,
		objectStore,
		newFakeQuotaManager(),
	)
	handler := NewHandler(service, publicKey)

	mux := http.NewServeMux()
	handler.Routes(mux)

	content := string(bytes.Repeat([]byte("a"), maxUploadSize+1))

	request := newUploadRequest(
		t,
		token,
		"preveliki.txt",
		content,
		"",
	)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusRequestEntityTooLarge,
			response.Code,
			response.Body.String(),
		)
	}

	if len(repository.files) != 0 {
		t.Errorf("expected no files in repository, got %d", len(repository.files))
	}

	if len(objectStore.objects) != 0 {
		t.Errorf("expected no objects in storage, got %d", len(objectStore.objects))
	}
}

func TestUploadFileWithoutAuthorization(t *testing.T) {
	handler := NewHandler(nil, nil)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := newUploadRequest(
		t,
		"",
		"dokument.txt",
		"sadrzaj",
		"",
	)

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

func TestUploadFileWithInvalidParentID(t *testing.T) {
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
	objectStore := newFakeObjectStorage()
	service := NewServiceWithDependencies(
		repository,
		objectStore,
		newFakeQuotaManager(),
	)
	handler := NewHandler(service, publicKey)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := newUploadRequest(
		t,
		token,
		"dokument.txt",
		"sadrzaj",
		"neispravan-id",
	)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusBadRequest,
			response.Code,
			response.Body.String(),
		)
	}

	if len(repository.files) != 0 {
		t.Errorf("expected no files in repository, got %d", len(repository.files))
	}

	if len(objectStore.objects) != 0 {
		t.Errorf("expected no objects in storage, got %d", len(objectStore.objects))
	}
}

func TestUploadFileIntoAnotherUsersFolder(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}

	userID := bson.NewObjectID()
	anotherUserID := bson.NewObjectID()

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

	foreignFolder := &Folder{
		ID:        bson.NewObjectID(),
		OwnerID:   anotherUserID,
		Name:      "Tudji folder",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repository.folders[foreignFolder.ID] = foreignFolder

	objectStore := newFakeObjectStorage()
	service := NewServiceWithDependencies(repository, objectStore, newFakeQuotaManager())
	handler := NewHandler(service, publicKey)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := newUploadRequest(
		t,
		token,
		"dokument.txt",
		"sadrzaj",
		foreignFolder.ID.Hex(),
	)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusNotFound,
			response.Code,
			response.Body.String(),
		)
	}

	if len(repository.files) != 0 {
		t.Errorf("expected no files in repository, got %d", len(repository.files))
	}

	if len(objectStore.objects) != 0 {
		t.Errorf("expected uploaded object to be removed, got %d objects", len(objectStore.objects))
	}
}

func TestUploadFileWithoutFile(t *testing.T) {
	handler, token := createTestHandler(t)

	mux := http.NewServeMux()
	handler.Routes(mux)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("parentId", ""); err != nil {
		t.Fatalf("write parentId: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/files",
		&body,
	)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			response.Code,
		)
	}
}

func TestUploadFileWithMultipleFiles(t *testing.T) {
	handler, token := createTestHandler(t)

	mux := http.NewServeMux()
	handler.Routes(mux)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for _, filename := range []string{"prvi.txt", "drugi.txt"} {
		filePart, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("create multipart file: %v", err)
		}

		if _, err := filePart.Write([]byte("test content")); err != nil {
			t.Fatalf("write file content: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/files",
		&body,
	)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusBadRequest,
			response.Code,
			response.Body.String(),
		)
	}
}

func TestUploadFileVersionThroughHTTPSuccessfully(t *testing.T) {
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
	objectStore := newFakeObjectStorage()
	service := NewServiceWithDependencies(
		repository,
		objectStore,
		newFakeQuotaManager(),
	)
	handler := NewHandler(service, publicKey)

	mux := http.NewServeMux()
	handler.Routes(mux)

	// First upload the original file.
	oldContent := "Originalni sadrzaj"
	request := newUploadRequest(
		t,
		token,
		"dokument.txt",
		oldContent,
		"",
	)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"initial upload: expected status %d, got %d: %s",
			http.StatusCreated,
			response.Code,
			response.Body.String(),
		)
	}

	var originalFile File
	if err := json.NewDecoder(response.Body).Decode(&originalFile); err != nil {
		t.Fatalf("decode original file: %v", err)
	}

	var originalVersion *FileVersion
	for _, version := range repository.fileVersions {
		originalVersion = version
	}

	if originalVersion == nil {
		t.Fatal("expected original file version")
	}

	oldObjectKey := originalVersion.ObjectKey

	// Upload a new version of the existing file.
	newContent := "Novi sadrzaj dokumenta"
	request = newUploadRequest(
		t,
		token,
		"dokument.txt",
		newContent,
		"",
	)

	request.URL.Path = "/files/" + originalFile.ID.Hex() + "/versions"

	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"version upload: expected status %d, got %d: %s",
			http.StatusCreated,
			response.Code,
			response.Body.String(),
		)
	}

	var updatedFile File
	if err := json.NewDecoder(response.Body).Decode(&updatedFile); err != nil {
		t.Fatalf("decode updated file: %v", err)
	}

	if updatedFile.ID != originalFile.ID {
		t.Error("file ID should remain unchanged")
	}

	if updatedFile.OwnerID != userID {
		t.Error("file owner should remain unchanged")
	}

	if updatedFile.CurrentVersion != 2 {
		t.Errorf(
			"expected current version 2, got %d",
			updatedFile.CurrentVersion,
		)
	}

	if len(repository.files) != 1 {
		t.Errorf(
			"expected 1 file in repository, got %d",
			len(repository.files),
		)
	}

	if len(repository.fileVersions) != 2 {
		t.Fatalf(
			"expected 2 file versions, got %d",
			len(repository.fileVersions),
		)
	}

	var newVersion *FileVersion

	for _, version := range repository.fileVersions {
		if version.VersionNumber == 2 {
			newVersion = version
			break
		}
	}

	if newVersion == nil {
		t.Fatal("expected second file version")
	}

	if newVersion.FileID != originalFile.ID {
		t.Error("new version references incorrect file ID")
	}

	if newVersion.SizeBytes != int64(len(newContent)) {
		t.Errorf(
			"expected size %d, got %d",
			len(newContent),
			newVersion.SizeBytes,
		)
	}

	if newVersion.ObjectKey == oldObjectKey {
		t.Error("new version should have a different object key")
	}

	if objectStore.objects[oldObjectKey] != oldContent {
		t.Error("original file content should remain unchanged")
	}

	if objectStore.objects[newVersion.ObjectKey] != newContent {
		t.Error("new file content was not stored correctly")
	}

	if len(objectStore.objects) != 2 {
		t.Errorf(
			"expected 2 objects in storage, got %d",
			len(objectStore.objects),
		)
	}
}

func TestUploadFileVersionRejectsAnotherUsersFile(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}

	userID := bson.NewObjectID()
	anotherUserID := bson.NewObjectID()

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
	objectStore := newFakeObjectStorage()

	now := time.Now().UTC()
	fileID := bson.NewObjectID()

	file := &File{
		ID:             fileID,
		OwnerID:        anotherUserID,
		Name:           "tudji-dokument.txt",
		CurrentVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	repository.files[fileID] = file

	version := &FileVersion{
		ID:            bson.NewObjectID(),
		FileID:        fileID,
		VersionNumber: 1,
		ObjectKey:     "originalni-objekat",
		SizeBytes:     10,
		ContentType:   "text/plain",
		CreatedAt:     now,
	}
	repository.fileVersions[version.ID] = version
	objectStore.objects[version.ObjectKey] = "old content"

	service := NewServiceWithDependencies(
		repository,
		objectStore,
		newFakeQuotaManager(),
	)
	handler := NewHandler(service, publicKey)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := newUploadRequest(
		t,
		token,
		"dokument.txt",
		"novi sadrzaj",
		"",
	)
	request.URL.Path = "/files/" + fileID.Hex() + "/versions"

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusNotFound,
			response.Code,
			response.Body.String(),
		)
	}

	if file.CurrentVersion != 1 {
		t.Errorf("expected current version 1, got %d", file.CurrentVersion)
	}

	if len(repository.fileVersions) != 1 {
		t.Errorf("expected 1 version, got %d", len(repository.fileVersions))
	}

	if len(objectStore.objects) != 1 {
		t.Errorf("expected original object only, got %d objects", len(objectStore.objects))
	}
}

func TestUploadFileVersionWithInvalidFileID(t *testing.T) {
	handler, token := createTestHandler(t)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := newUploadRequest(
		t,
		token,
		"dokument.txt",
		"novi sadrzaj",
		"",
	)
	request.URL.Path = "/files/neispravan-id/versions"

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusBadRequest,
			response.Code,
			response.Body.String(),
		)
	}
}

func TestUploadFileVersionWithoutAuthorization(t *testing.T) {
	handler := NewHandler(nil, nil)

	mux := http.NewServeMux()
	handler.Routes(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/files/"+bson.NewObjectID().Hex()+"/versions",
		nil,
	)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			response.Code,
		)
	}
}

func TestUploadFileVersionWithoutFile(t *testing.T) {
	handler, token := createTestHandler(t)

	mux := http.NewServeMux()
	handler.Routes(mux)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/files/"+bson.NewObjectID().Hex()+"/versions",
		&body,
	)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusBadRequest,
			response.Code,
			response.Body.String(),
		)
	}
}

func TestUploadFileVersionRejectsMultipleFiles(t *testing.T) {
	handler, token := createTestHandler(t)

	mux := http.NewServeMux()
	handler.Routes(mux)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for _, filename := range []string{"prvi.txt", "drugi.txt"} {
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}

		if _, err := part.Write([]byte("test content")); err != nil {
			t.Fatalf("write file content: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/files/"+bson.NewObjectID().Hex()+"/versions",
		&body,
	)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusBadRequest,
			response.Code,
			response.Body.String(),
		)
	}
}

func TestUploadFileVersionTooLarge(t *testing.T) {
	handler, token := createTestHandler(t)

	mux := http.NewServeMux()
	handler.Routes(mux)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", "large.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}

	content := bytes.Repeat([]byte("a"), maxUploadSize+1)
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write file content: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/files/"+bson.NewObjectID().Hex()+"/versions",
		&body,
	)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusRequestEntityTooLarge,
			response.Code,
			response.Body.String(),
		)
	}
}
