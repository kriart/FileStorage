package api

import (
	"net/http"

	"file-storage-server/internal/access"
	"file-storage-server/internal/audit"
	"file-storage-server/internal/httpx"
	"file-storage-server/internal/middleware"
	"file-storage-server/internal/repository"

	"github.com/go-chi/chi/v5"
)

type AccessHandler struct {
	accesses *access.StorageAccessService
	audit    *audit.Service
}

func NewAccessHandler(accesses *access.StorageAccessService, auditService *audit.Service) *AccessHandler {
	return &AccessHandler{accesses: accesses, audit: auditService}
}

func (h *AccessHandler) ListStorageAccesses(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	storageID, err := parseID(chi.URLParam(r, "storageId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	response, err := h.accesses.List(r.Context(), currentUser.ID, storageID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *AccessHandler) GrantStorageAccess(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	storageID, err := parseID(chi.URLParam(r, "storageId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	dto := access.GrantStorageAccessDTO{}
	if err := httpx.DecodeJSON(r, &dto); err != nil {
		httpx.WriteError(w, err)
		return
	}

	response, err := h.accesses.Grant(r.Context(), currentUser.ID, storageID, dto)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	auditLog(r, h.audit, currentUser, "access.grant", "storage", storageID, map[string]any{"targetUserId": response.UserID, "targetEmail": response.UserEmail, "level": response.AccessLevel})
	httpx.WriteJSON(w, http.StatusCreated, response)
}

func (h *AccessHandler) UpdateStorageAccess(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	storageID, err := parseID(chi.URLParam(r, "storageId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	userID, err := parseID(chi.URLParam(r, "userId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	dto := access.UpdateStorageAccessDTO{}
	if err := httpx.DecodeJSON(r, &dto); err != nil {
		httpx.WriteError(w, err)
		return
	}

	response, err := h.accesses.Update(r.Context(), currentUser.ID, storageID, userID, dto)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	auditLog(r, h.audit, currentUser, "access.update", "storage", storageID, map[string]any{"targetUserId": response.UserID, "level": response.AccessLevel})
	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *AccessHandler) DeleteStorageAccess(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	storageID, err := parseID(chi.URLParam(r, "storageId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	userID, err := parseID(chi.URLParam(r, "userId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	if err := h.accesses.Delete(r.Context(), currentUser.ID, storageID, userID); err != nil {
		httpx.WriteError(w, err)
		return
	}

	auditLog(r, h.audit, currentUser, "access.delete", "storage", storageID, map[string]any{"targetUserId": userID})
	httpx.WriteNoContent(w)
}
