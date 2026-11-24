package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"crypto/ed25519"

	"github.com/acilerts/cloud-drive/internal/platform"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	maxUploadSize = 50 << 20 // 50 MiB
	maxFormMemory = 10 << 20 // 10 MiB
)

type Handler struct {
	service   *Service
	publicKey ed25519.PublicKey
}

func NewHandler(service *Service, publicKey ed25519.PublicKey) *Handler {
	return &Handler{
		service:   service,
		publicKey: publicKey,
	}
}

type createFolderRequest struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parentId"`
}

type renameRequest struct {
	Name string `json:"name"`
}

func (handler *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /folders", handler.createFolder)
	mux.HandleFunc("GET /folders", handler.listFolders)
	mux.HandleFunc("GET /folders/contents", handler.listFolderContents)
	mux.HandleFunc("PATCH /folders/{folderId}", handler.renameFolder)
	mux.HandleFunc("PATCH /files/{fileId}", handler.renameFile)
	mux.HandleFunc("POST /files", handler.uploadFile)
	mux.HandleFunc("POST /files/{fileId}/versions", handler.uploadFileVersion)
}

func (handler *Handler) createFolder(
	w http.ResponseWriter,
	r *http.Request,
) {
	ownerID, err := handler.authenticate(r)
	if err != nil {
		platform.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var request createFolderRequest

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		platform.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		platform.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var parentID *bson.ObjectID

	if request.ParentID != nil {
		id, err := bson.ObjectIDFromHex(*request.ParentID)
		if err != nil {
			platform.WriteError(w, http.StatusBadRequest, "invalid parentId")
			return
		}

		parentID = &id
	}

	folder, err := handler.service.CreateFolder(
		r.Context(),
		ownerID,
		request.Name,
		parentID,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidFolderName):
			platform.WriteError(w, http.StatusBadRequest, "folder name is required")

		case errors.Is(err, ErrParentNotFound):
			platform.WriteError(w, http.StatusNotFound, "parent folder not found")

		default:
			platform.WriteError(w, http.StatusInternalServerError, "could not create folder")
		}
		return
	}

	platform.WriteJSON(w, http.StatusCreated, folder)
}

func (handler *Handler) authenticate(
	r *http.Request,
) (bson.ObjectID, error) {
	header := r.Header.Get("Authorization")
	parts := strings.Fields(header)

	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return bson.NilObjectID, errors.New("missing bearer token")
	}

	claims, err := platform.VerifyToken(handler.publicKey, parts[1])
	if err != nil {
		return bson.NilObjectID, errors.New("invalid token")
	}

	if claims.Role != "user" {
		return bson.ObjectID{}, errors.New("user role required")
	}

	userID, err := bson.ObjectIDFromHex(claims.Subject)
	if err != nil {
		return bson.NilObjectID, errors.New("invalid user ID in token")
	}

	return userID, nil
}

func (handler *Handler) listFolders(
	w http.ResponseWriter,
	r *http.Request,
) {
	ownerID, err := handler.authenticate(r)
	if err != nil {
		platform.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var parentID *bson.ObjectID

	parentIDValue := r.URL.Query().Get("parentId")

	if parentIDValue != "" {
		id, err := bson.ObjectIDFromHex(parentIDValue)
		if err != nil {
			platform.WriteError(w, http.StatusBadRequest, "invalid parentId")
			return
		}

		parentID = &id
	}

	folders, err := handler.service.ListFolders(
		r.Context(),
		ownerID,
		parentID,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrParentNotFound):
			platform.WriteError(w, http.StatusNotFound, "parent folder not found")

		default:
			platform.WriteError(w, http.StatusInternalServerError, "could not list folders")
		}

		return
	}

	platform.WriteJSON(w, http.StatusOK, folders)
}

func (handler *Handler) listFolderContents(
	w http.ResponseWriter,
	r *http.Request,
) {
	ownerID, err := handler.authenticate(r)
	if err != nil {
		platform.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var parentID *bson.ObjectID

	parentIDValue := r.URL.Query().Get("parentId")

	if parentIDValue != "" {
		id, err := bson.ObjectIDFromHex(parentIDValue)
		if err != nil {
			platform.WriteError(w, http.StatusBadRequest, "invalid parentId")
			return
		}

		parentID = &id
	}

	contents, err := handler.service.ListFolderContents(
		r.Context(),
		ownerID,
		parentID,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrParentNotFound):
			platform.WriteError(w, http.StatusNotFound, "parent folder not found")

		default:
			platform.WriteError(w, http.StatusInternalServerError, "could not list folder contents")
		}

		return
	}

	platform.WriteJSON(w, http.StatusOK, contents)
}

func (handler *Handler) renameFolder(
	w http.ResponseWriter,
	r *http.Request,
) {
	ownerID, err := handler.authenticate(r)
	if err != nil {
		platform.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	folderID, err := bson.ObjectIDFromHex(r.PathValue("folderId"))
	if err != nil {
		platform.WriteError(w, http.StatusBadRequest, "invalid folder ID")
		return
	}

	var request renameRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		platform.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := handler.service.RenameFolder(
		r.Context(),
		ownerID,
		folderID,
		request.Name,
	); err != nil {
		switch {
		case errors.Is(err, ErrInvalidFolderName):
			platform.WriteError(w, http.StatusBadRequest, "invalid folder name")

		case errors.Is(err, ErrParentNotFound):
			platform.WriteError(w, http.StatusNotFound, "folder not found")

		default:
			platform.WriteError(w, http.StatusInternalServerError, "could not rename folder")
		}

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) renameFile(
	w http.ResponseWriter,
	r *http.Request,
) {
	ownerID, err := handler.authenticate(r)
	if err != nil {
		platform.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	fileID, err := bson.ObjectIDFromHex(r.PathValue("fileId"))
	if err != nil {
		platform.WriteError(w, http.StatusBadRequest, "invalid file ID")
		return
	}

	var request renameRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		platform.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := handler.service.RenameFile(
		r.Context(),
		ownerID,
		fileID,
		request.Name,
	); err != nil {
		switch {
		case errors.Is(err, ErrInvalidFileName):
			platform.WriteError(w, http.StatusBadRequest, "invalid file name")

		case errors.Is(err, ErrFileNotFound):
			platform.WriteError(w, http.StatusNotFound, "file not found")

		default:
			platform.WriteError(w, http.StatusInternalServerError, "could not rename file")
		}

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) uploadFile(
	w http.ResponseWriter,
	r *http.Request,
) {
	ownerID, err := handler.authenticate(r)
	if err != nil {
		platform.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+(1<<20))

	if err := r.ParseMultipartForm(maxFormMemory); err != nil {
		var maxBytesError *http.MaxBytesError

		if errors.As(err, &maxBytesError) {
			platform.WriteError(w, http.StatusRequestEntityTooLarge, "request too large")
			return
		}

		platform.WriteError(w, http.StatusBadRequest, "invalid multipart request")
		return
	}

	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	fileHeaders := r.MultipartForm.File["file"]

	if len(fileHeaders) != 1 {
		platform.WriteError(w, http.StatusBadRequest, "exactly one file is required")
		return
	}

	fileHeader := fileHeaders[0]

	if fileHeader.Size > maxUploadSize {
		platform.WriteError(w, http.StatusRequestEntityTooLarge, "file too large")
		return
	}

	name := strings.TrimSpace(fileHeader.Filename)
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))

	if name == "" || name == "." {
		platform.WriteError(w, http.StatusBadRequest, "invalid file name")
		return
	}

	var parentID *bson.ObjectID

	parentValues := r.MultipartForm.Value["parentId"]

	if len(parentValues) > 1 {
		platform.WriteError(w, http.StatusBadRequest, "invalid parentId")
		return
	}

	if len(parentValues) == 1 && parentValues[0] != "" {
		id, err := bson.ObjectIDFromHex(parentValues[0])
		if err != nil {
			platform.WriteError(w, http.StatusBadRequest, "invalid parentId")
			return
		}

		parentID = &id
	}

	uploadedFile, err := fileHeader.Open()
	if err != nil {
		platform.WriteError(w, http.StatusInternalServerError, "could not read uploaded file")
		return
	}
	defer uploadedFile.Close()

	sample := make([]byte, 512)
	n, readErr := io.ReadFull(uploadedFile, sample)

	if readErr != nil && !errors.Is(readErr, io.EOF) &&
		!errors.Is(readErr, io.ErrUnexpectedEOF) {
		platform.WriteError(w, http.StatusBadRequest, "could not read uploaded file")
		return
	}

	contentType := http.DetectContentType(sample[:n])

	if _, err := uploadedFile.Seek(0, io.SeekStart); err != nil {
		platform.WriteError(w, http.StatusInternalServerError, "could not process uploaded file")
		return
	}

	objectKey := ownerID.Hex() + "/" + bson.NewObjectID().Hex()

	file, err := handler.service.UploadFile(
		r.Context(),
		ownerID,
		name,
		parentID,
		objectKey,
		uploadedFile,
		fileHeader.Size,
		contentType,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidFileName):
			platform.WriteError(w, http.StatusBadRequest, "invalid file name")

		case errors.Is(err, ErrInvalidFileSize):
			platform.WriteError(w, http.StatusBadRequest, "invalid file size")

		case errors.Is(err, ErrParentNotFound):
			platform.WriteError(w, http.StatusNotFound, "parent folder not found")

		default:
			platform.WriteError(w, http.StatusInternalServerError, "could not upload file")
		}

		return
	}

	platform.WriteJSON(w, http.StatusCreated, file)
}

func (handler *Handler) uploadFileVersion(
	w http.ResponseWriter,
	r *http.Request,
) {
	ownerID, err := handler.authenticate(r)
	if err != nil {
		platform.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	fileID, err := bson.ObjectIDFromHex(r.PathValue("fileId"))
	if err != nil {
		platform.WriteError(w, http.StatusBadRequest, "invalid fileId")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+(1<<20))

	if err := r.ParseMultipartForm(maxFormMemory); err != nil {
		var maxBytesError *http.MaxBytesError

		if errors.As(err, &maxBytesError) {
			platform.WriteError(w, http.StatusRequestEntityTooLarge, "request too large")
			return
		}

		platform.WriteError(w, http.StatusBadRequest, "invalid multipart request")
		return
	}

	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	fileHeaders := r.MultipartForm.File["file"]

	if len(fileHeaders) != 1 {
		platform.WriteError(w, http.StatusBadRequest, "exactly one file is required")
		return
	}

	fileHeader := fileHeaders[0]

	if fileHeader.Size > maxUploadSize {
		platform.WriteError(w, http.StatusRequestEntityTooLarge, "file too large")
		return
	}

	name := strings.TrimSpace(fileHeader.Filename)
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))

	if name == "" || name == "." {
		platform.WriteError(w, http.StatusBadRequest, "invalid file name")
		return
	}

	uploadedFile, err := fileHeader.Open()
	if err != nil {
		platform.WriteError(w, http.StatusInternalServerError, "could not read uploaded file")
		return
	}
	defer uploadedFile.Close()

	sample := make([]byte, 512)
	n, readErr := io.ReadFull(uploadedFile, sample)

	if readErr != nil && !errors.Is(readErr, io.EOF) &&
		!errors.Is(readErr, io.ErrUnexpectedEOF) {
		platform.WriteError(w, http.StatusBadRequest, "could not read uploaded file")
		return
	}

	contentType := http.DetectContentType(sample[:n])

	if _, err := uploadedFile.Seek(0, io.SeekStart); err != nil {
		platform.WriteError(w, http.StatusInternalServerError, "could not process uploaded file")
		return
	}

	objectKey := ownerID.Hex() + "/" + fileID.Hex() + "/" + bson.NewObjectID().Hex()

	file, err := handler.service.UploadFileVersion(
		r.Context(),
		ownerID,
		fileID,
		objectKey,
		uploadedFile,
		fileHeader.Size,
		contentType,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrFileNotFound):
			platform.WriteError(w, http.StatusNotFound, "file not found")

		case errors.Is(err, ErrInvalidFileSize):
			platform.WriteError(w, http.StatusBadRequest, "invalid file size")

		default:
			platform.WriteError(w, http.StatusInternalServerError, "could not upload file version")
		}

		return
	}

	platform.WriteJSON(w, http.StatusCreated, file)
}
