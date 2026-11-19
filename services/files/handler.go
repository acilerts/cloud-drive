package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"crypto/ed25519"

	"github.com/acilerts/cloud-drive/internal/platform"
	"go.mongodb.org/mongo-driver/v2/bson"
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

func (handler *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /folders", handler.createFolder)
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
