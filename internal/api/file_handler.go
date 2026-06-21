package api

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"file-storage-server/internal/audit"
	"file-storage-server/internal/file"
	"file-storage-server/internal/httpx"
	"file-storage-server/internal/middleware"
	"file-storage-server/internal/repository"

	"github.com/go-chi/chi/v5"
)

type FileHandler struct {
	files *file.Service
	audit *audit.Service
}

const maxMultipartBodySize = 512 << 20

func NewFileHandler(files *file.Service, auditService *audit.Service) *FileHandler {
	return &FileHandler{files: files, audit: auditService}
}

func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
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

	r.Body = http.MaxBytesReader(w, r.Body, maxMultipartBodySize)
	src, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, fmt.Errorf("%w: file field is required", repository.ErrInvalidInput))
		return
	}
	defer src.Close()

	folderID, err := parseOptionalID(r.FormValue("folderId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	response, err := h.files.Upload(r.Context(), currentUser.ID, storageID, folderID, header.Filename, src)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	auditLog(r, h.audit, currentUser, "file.upload", "file", response.ID, map[string]any{"storageId": storageID, "name": response.OriginalName, "size": response.Size})
	httpx.WriteJSON(w, http.StatusCreated, response)
}

func (h *FileHandler) List(w http.ResponseWriter, r *http.Request) {
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

	folderID, err := parseOptionalID(r.URL.Query().Get("folderId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	response, err := h.files.List(r.Context(), currentUser.ID, storageID, folderID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *FileHandler) Get(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	fileID, err := parseID(chi.URLParam(r, "fileId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	response, err := h.files.Get(r.Context(), currentUser.ID, fileID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *FileHandler) Download(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	fileID, err := parseID(chi.URLParam(r, "fileId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	download, err := h.files.Download(r.Context(), currentUser.ID, fileID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	defer download.Reader.Close()

	w.Header().Set("Content-Type", download.File.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(download.File.Size, 10))
	w.Header().Set("Content-Disposition", contentDisposition(download.File.OriginalName))
	w.WriteHeader(http.StatusOK)
	auditLog(r, h.audit, currentUser, "file.download", "file", fileID, map[string]any{"storageId": download.File.StorageID})
	_, _ = io.Copy(w, download.Reader)
}

func (h *FileHandler) Replace(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	fileID, err := parseID(chi.URLParam(r, "fileId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxMultipartBodySize)
	src, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, fmt.Errorf("%w: file field is required", repository.ErrInvalidInput))
		return
	}
	defer src.Close()

	response, err := h.files.Replace(r.Context(), currentUser.ID, fileID, header.Filename, src)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	auditLog(r, h.audit, currentUser, "file.replace", "file", response.ID, map[string]any{"name": response.OriginalName, "size": response.Size})
	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *FileHandler) Rename(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	fileID, err := parseID(chi.URLParam(r, "fileId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	dto := file.RenameFileDTO{}
	if err := httpx.DecodeJSON(r, &dto); err != nil {
		httpx.WriteError(w, err)
		return
	}

	response, err := h.files.Rename(r.Context(), currentUser.ID, fileID, dto)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	auditLog(r, h.audit, currentUser, "file.rename", "file", response.ID, map[string]any{"name": response.OriginalName})
	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	fileID, err := parseID(chi.URLParam(r, "fileId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	if err := h.files.Delete(r.Context(), currentUser.ID, fileID); err != nil {
		httpx.WriteError(w, err)
		return
	}

	auditLog(r, h.audit, currentUser, "file.delete", "file", fileID, nil)
	httpx.WriteNoContent(w)
}

func contentDisposition(filename string) string {
	filename = strings.ReplaceAll(filename, `"`, `'`)
	return fmt.Sprintf(`attachment; filename="%s"`, filename)
}
