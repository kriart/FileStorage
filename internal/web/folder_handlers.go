package web

import (
	"net/http"

	"file-storage-server/internal/folder"
	"file-storage-server/internal/middleware"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) CreateFolder(w http.ResponseWriter, r *http.Request) {
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
	if err := r.ParseForm(); err != nil {
		flashError(w, "Не удалось прочитать форму папки")
		redirectToStorageFolder(w, r, storageID, nil)
		return
	}

	parentID, err := parseOptionalID(r.FormValue("parentId"))
	if err != nil {
		flashError(w, "Некорректная родительская папка")
		redirectToStorageFolder(w, r, storageID, nil)
		return
	}

	created, err := h.folders.Create(r.Context(), currentUser.ID, folder.CreateFolderDTO{
		StorageID: storageID,
		ParentID:  parentID,
		Name:      r.FormValue("name"),
	})
	if err != nil {
		flashError(w, "Не удалось создать папку")
		redirectToStorageFolder(w, r, storageID, parentID)
		return
	}
	flashSuccess(w, "Папка создана")
	h.log(r, currentUser.ID, "folder.create", "folder", created.ID, map[string]any{"storageId": storageID, "name": created.Name})
	redirectToStorageFolder(w, r, storageID, parentID)
}

func (h *Handler) RenameFolder(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	folderID, err := parseID(chi.URLParam(r, "folderId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		flashError(w, "Не удалось прочитать форму переименования")
		redirectBack(w, r, "/")
		return
	}

	renamed, err := h.folders.Rename(r.Context(), currentUser.ID, folderID, folder.RenameFolderDTO{
		Name: r.FormValue("name"),
	})
	if err != nil {
		flashError(w, "Не удалось переименовать папку")
		redirectBack(w, r, "/")
		return
	}
	flashSuccess(w, "Папка переименована")
	h.log(r, currentUser.ID, "folder.rename", "folder", renamed.ID, map[string]any{"name": renamed.Name})
	redirectBack(w, r, "/")
}

func (h *Handler) DeleteFolder(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	folderID, err := parseID(chi.URLParam(r, "folderId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	folderDTO, err := h.folders.Get(r.Context(), currentUser.ID, folderID)
	if err != nil {
		flashError(w, "Не удалось найти папку")
		redirectBack(w, r, "/")
		return
	}
	parentID := folderDTO.ParentID

	deleted, err := h.folders.Delete(r.Context(), currentUser.ID, folderID)
	if err != nil {
		flashError(w, "Не удалось удалить папку")
		redirectBack(w, r, "/")
		return
	}
	flashSuccess(w, "Папка удалена")
	h.log(r, currentUser.ID, "folder.delete", "folder", folderID, map[string]any{"storageId": deleted.StorageID})
	redirectToStorageFolder(w, r, deleted.StorageID, parentID)
}
