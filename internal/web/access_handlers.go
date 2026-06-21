package web

import (
	"net/http"
	"strconv"

	"file-storage-server/internal/access"
	"file-storage-server/internal/domain"
	"file-storage-server/internal/middleware"
	"file-storage-server/internal/repository"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) GrantAccess(w http.ResponseWriter, r *http.Request) {
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
		flashError(w, "Не удалось прочитать форму доступа")
		http.Redirect(w, r, "/storages/"+strconv.FormatInt(storageID, 10), http.StatusSeeOther)
		return
	}
	if _, err := h.accesses.Grant(r.Context(), currentUser.ID, storageID, access.GrantStorageAccessDTO{
		UserEmail:   r.FormValue("userEmail"),
		AccessLevel: domain.StorageAccessLevel(r.FormValue("accessLevel")),
	}); err != nil {
		flashError(w, "Не удалось выдать доступ")
		http.Redirect(w, r, "/storages/"+strconv.FormatInt(storageID, 10), http.StatusSeeOther)
		return
	}
	flashSuccess(w, "Доступ выдан")
	h.log(r, currentUser.ID, "access.grant", "storage", storageID, map[string]any{"targetUserId": r.FormValue("userId"), "targetEmail": r.FormValue("userEmail"), "level": r.FormValue("accessLevel")})
	http.Redirect(w, r, "/storages/"+strconv.FormatInt(storageID, 10), http.StatusSeeOther)
}

func (h *Handler) DeleteAccess(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	storageID, err := parseID(chi.URLParam(r, "storageId"))
	userID, userErr := parseID(chi.URLParam(r, "userId"))
	if err == nil && userErr == nil {
		if err := h.accesses.Delete(r.Context(), currentUser.ID, storageID, userID); err != nil {
			flashError(w, "Не удалось убрать доступ")
			http.Redirect(w, r, "/storages/"+chi.URLParam(r, "storageId"), http.StatusSeeOther)
			return
		}
		flashSuccess(w, "Доступ убран")
		h.log(r, currentUser.ID, "access.delete", "storage", storageID, map[string]any{"targetUserId": userID})
	}
	http.Redirect(w, r, "/storages/"+chi.URLParam(r, "storageId"), http.StatusSeeOther)
}

func (h *Handler) GrantFolderAccess(w http.ResponseWriter, r *http.Request) {
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
		flashError(w, "Не удалось прочитать форму доступа к папке")
		redirectBack(w, r, "/")
		return
	}
	granted, err := h.accesses.GrantFolder(r.Context(), currentUser.ID, folderID, access.GrantStorageAccessDTO{
		UserEmail:   r.FormValue("userEmail"),
		AccessLevel: domain.StorageAccessLevel(r.FormValue("accessLevel")),
	})
	if err != nil {
		flashError(w, repository.PublicMessage(err))
		redirectBack(w, r, "/")
		return
	}
	flashSuccess(w, "Доступ к папке выдан")
	h.log(r, currentUser.ID, "folder_access.grant", "folder", folderID, map[string]any{"targetUserId": granted.UserID, "targetEmail": granted.UserEmail, "level": granted.AccessLevel})
	redirectBack(w, r, "/")
}

func (h *Handler) DeleteFolderAccess(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	folderID, err := parseID(chi.URLParam(r, "folderId"))
	userID, userErr := parseID(chi.URLParam(r, "userId"))
	if err == nil && userErr == nil {
		if err := h.accesses.DeleteFolder(r.Context(), currentUser.ID, folderID, userID); err != nil {
			flashError(w, "Не удалось убрать доступ к папке")
			redirectBack(w, r, "/")
			return
		}
		flashSuccess(w, "Доступ к папке убран")
		h.log(r, currentUser.ID, "folder_access.delete", "folder", folderID, map[string]any{"targetUserId": userID})
	}
	redirectBack(w, r, "/")
}
