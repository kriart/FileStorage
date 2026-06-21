package api

import (
	"net/http"

	"file-storage-server/internal/audit"
	"file-storage-server/internal/httpx"
	"file-storage-server/internal/middleware"
	"file-storage-server/internal/repository"
	"file-storage-server/internal/storage"

	"github.com/go-chi/chi/v5"
)

type StorageHandler struct {
	storages *storage.Service
	audit    *audit.Service
}

func NewStorageHandler(storages *storage.Service, auditService *audit.Service) *StorageHandler {
	return &StorageHandler{storages: storages, audit: auditService}
}

func (h *StorageHandler) Create(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	dto := storage.CreateStorageDTO{}
	if err := httpx.DecodeJSON(r, &dto); err != nil {
		httpx.WriteError(w, err)
		return
	}

	response, err := h.storages.Create(r.Context(), currentUser.ID, currentUser.Role, dto)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	auditLog(r, h.audit, currentUser, "storage.create", "storage", response.ID, map[string]any{"name": response.Name})
	httpx.WriteJSON(w, http.StatusCreated, response)
}

func (h *StorageHandler) List(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	response, err := h.storages.List(r.Context(), currentUser.ID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *StorageHandler) Get(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	id, err := parseID(chi.URLParam(r, "storageId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	response, err := h.storages.Get(r.Context(), currentUser.ID, id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	auditLog(r, h.audit, currentUser, "storage.update", "storage", response.ID, nil)
	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *StorageHandler) Update(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	id, err := parseID(chi.URLParam(r, "storageId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	dto := storage.UpdateStorageDTO{}
	if err := httpx.DecodeJSON(r, &dto); err != nil {
		httpx.WriteError(w, err)
		return
	}

	response, err := h.storages.Update(r.Context(), currentUser.ID, id, dto)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *StorageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	id, err := parseID(chi.URLParam(r, "storageId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	if err := h.storages.Delete(r.Context(), currentUser.ID, id); err != nil {
		httpx.WriteError(w, err)
		return
	}

	auditLog(r, h.audit, currentUser, "storage.delete", "storage", id, nil)
	httpx.WriteNoContent(w)
}
