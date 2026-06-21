package web

import (
	"net/http"

	"file-storage-server/internal/file"
	"file-storage-server/internal/middleware"
	"file-storage-server/internal/repository"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	storageID, err := parseID(chi.URLParam(r, "storageId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 512<<20)

	src, header, err := r.FormFile("file")
	if err == nil {
		defer src.Close()
		folderID, _ := parseOptionalID(r.FormValue("folderId"))
		uploaded, err := h.files.Upload(r.Context(), currentUser.ID, storageID, folderID, header.Filename, src)
		if err != nil {
			flashError(w, uploadErrorMessage(err))
			redirectToStorageFolder(w, r, storageID, folderID)
			return
		}
		flashSuccess(w, "Файл добавлен")
		h.log(r, currentUser.ID, "file.upload", "file", uploaded.ID, map[string]any{"storageId": storageID, "name": uploaded.OriginalName, "size": uploaded.Size})
	} else {
		flashError(w, "Не удалось прочитать файл")
	}
	folderID, _ := parseOptionalID(r.FormValue("folderId"))
	redirectToStorageFolder(w, r, storageID, folderID)
}

func (h *Handler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	fileID, err := parseID(chi.URLParam(r, "fileId"))
	if err == nil {
		if err := h.files.Delete(r.Context(), currentUser.ID, fileID); err != nil {
			flashError(w, "Не удалось удалить файл")
			redirectBack(w, r, "/")
			return
		}
		flashSuccess(w, "Файл удален")
		h.log(r, currentUser.ID, "file.delete", "file", fileID, nil)
	}
	redirectBack(w, r, "/")
}

func (h *Handler) RenameFile(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	fileID, err := parseID(chi.URLParam(r, "fileId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		flashError(w, "Не удалось прочитать форму переименования")
		redirectBack(w, r, "/")
		return
	}

	renamed, err := h.files.Rename(r.Context(), currentUser.ID, fileID, file.RenameFileDTO{Name: r.FormValue("name")})
	if err != nil {
		flashError(w, repository.PublicMessage(err))
		redirectBack(w, r, "/")
		return
	}
	flashSuccess(w, "Файл переименован")
	h.log(r, currentUser.ID, "file.rename", "file", renamed.ID, map[string]any{"name": renamed.OriginalName})
	redirectBack(w, r, "/")
}

func (h *Handler) ReplaceFile(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	fileID, err := parseID(chi.URLParam(r, "fileId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 512<<20)

	src, header, err := r.FormFile("file")
	if err == nil {
		defer src.Close()
		replaced, err := h.files.Replace(r.Context(), currentUser.ID, fileID, header.Filename, src)
		if err != nil {
			flashError(w, uploadErrorMessage(err))
			redirectBack(w, r, "/")
			return
		}
		flashSuccess(w, "Файл заменен")
		h.log(r, currentUser.ID, "file.replace", "file", replaced.ID, map[string]any{"name": replaced.OriginalName, "size": replaced.Size})
	} else {
		flashError(w, "Не удалось прочитать файл для замены")
	}
	redirectBack(w, r, "/")
}
