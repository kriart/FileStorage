package web

import (
	"net/http"
	"strconv"
	"strings"

	"file-storage-server/internal/access"
	"file-storage-server/internal/domain"
	"file-storage-server/internal/file"
	"file-storage-server/internal/folder"
	"file-storage-server/internal/middleware"
	"file-storage-server/internal/share"
	"file-storage-server/internal/storage"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) NewStoragePage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "storage_new.html", PageData{Title: "New storage", User: currentUser(r)})
}

func (h *Handler) CreateStorage(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.render(w, r, "storage_new.html", PageData{Title: "New storage", User: currentUser, Error: "Некорректная форма", Flash: &Flash{Kind: "error", Message: "Не удалось прочитать форму хранилища"}})
		return
	}

	maxFileSize, _ := strconv.ParseInt(r.FormValue("maxFileSize"), 10, 64)
	maxStorageSize, _ := strconv.ParseInt(r.FormValue("maxStorageSize"), 10, 64)
	created, err := h.storages.Create(r.Context(), currentUser.ID, currentUser.Role, storage.CreateStorageDTO{
		Name:             r.FormValue("name"),
		Type:             domain.StorageType(r.FormValue("type")),
		Visibility:       domain.StorageVisibility(r.FormValue("visibility")),
		MaxFileSize:      maxFileSize,
		MaxStorageSize:   maxStorageSize,
		AllowedFileTypes: splitMimeTypes(r.FormValue("allowedFileTypes")),
		BlockedFileTypes: splitMimeTypes(r.FormValue("blockedFileTypes")),
	})
	if err != nil {
		h.render(w, r, "storage_new.html", PageData{Title: "New storage", User: currentUser, Error: "Не удалось создать хранилище", Flash: &Flash{Kind: "error", Message: "Не удалось создать хранилище"}})
		return
	}

	flashSuccess(w, "Хранилище создано")
	h.log(r, currentUser.ID, "storage.create", "storage", created.ID, map[string]any{"name": created.Name})
	http.Redirect(w, r, "/storages/"+strconv.FormatInt(created.ID, 10), http.StatusSeeOther)
}

func (h *Handler) StorageDetail(w http.ResponseWriter, r *http.Request) {
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

	storageDTO, err := h.storages.Get(r.Context(), currentUser.ID, storageID)
	if err != nil {
		h.render(w, r, "storage_detail.html", PageData{Title: "Storage", User: currentUser, Error: "Не удалось загрузить хранилище"})
		return
	}

	folderID, err := parseOptionalID(r.URL.Query().Get("folderId"))
	if err != nil {
		http.Redirect(w, r, "/storages/"+strconv.FormatInt(storageID, 10), http.StatusSeeOther)
		return
	}

	fileSearch := strings.TrimSpace(r.URL.Query().Get("q"))
	fileSort := strings.TrimSpace(r.URL.Query().Get("sort"))
	fileDirection := strings.TrimSpace(r.URL.Query().Get("dir"))
	filePage := parsePage(r.URL.Query().Get("page"))
	fileLimit := 20
	fileOffset := (filePage - 1) * fileLimit
	filesPage, _ := h.files.ListPage(r.Context(), currentUser.ID, file.ListFilesFilter{
		StorageID: storageID,
		FolderID:  folderID,
		Search:    fileSearch,
		Sort:      fileSort,
		Direction: fileDirection,
		Limit:     fileLimit,
		Offset:    fileOffset,
	})
	files := []file.FileDTO{}
	fileTotal := 0
	if filesPage != nil {
		files = filesPage.Files
		fileTotal = filesPage.Total
	}
	folders, _ := h.folders.List(r.Context(), currentUser.ID, storageID, folderID)
	path, _ := h.folders.Path(r.Context(), currentUser.ID, storageID, folderID)
	accesses, _ := h.accesses.List(r.Context(), currentUser.ID, storageID)
	folderAccesses := make(map[int64][]access.FolderAccessDTO, len(folders))
	for _, folderDTO := range folders {
		if rows, err := h.accesses.ListFolder(r.Context(), currentUser.ID, folderDTO.ID); err == nil {
			folderAccesses[folderDTO.ID] = rows
		}
	}
	fileLinks := make(map[int64][]share.ShareLinkDTO, len(files))
	for _, fileDTO := range files {
		links, err := h.shares.List(r.Context(), currentUser.ID, fileDTO.ID, publicBaseURL(r))
		if err == nil {
			fileLinks[fileDTO.ID] = links
		}
	}
	var currentFolder *folder.FolderDTO
	if len(path) > 0 {
		currentFolder = &path[len(path)-1]
	}
	activeLinkFileID, _ := strconv.ParseInt(r.URL.Query().Get("linkFileId"), 10, 64)

	h.render(w, r, "storage_detail.html", PageData{
		Title:            storageDTO.Name,
		User:             currentUser,
		Storage:          storageDTO,
		FolderID:         folderID,
		Folder:           currentFolder,
		Folders:          folders,
		Path:             path,
		Files:            files,
		FileSearch:       fileSearch,
		FileSort:         fileSort,
		FileDirection:    fileDirection,
		FilePage:         filePage,
		FilePrevPage:     filePage - 1,
		FileNextPage:     filePage + 1,
		FileTotal:        fileTotal,
		FileHasPrev:      filePage > 1,
		FileHasNext:      fileOffset+len(files) < fileTotal,
		Accesses:         accesses,
		FolderAccesses:   folderAccesses,
		FileLinks:        fileLinks,
		ShareURL:         r.URL.Query().Get("createdLink"),
		ActiveLinkFileID: activeLinkFileID,
	})
}

func (h *Handler) DeleteStorage(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	storageID, err := parseID(chi.URLParam(r, "storageId"))
	if err == nil {
		if err := h.storages.Delete(r.Context(), currentUser.ID, storageID); err != nil {
			flashError(w, "Не удалось удалить хранилище")
			http.Redirect(w, r, "/storages/"+strconv.FormatInt(storageID, 10), http.StatusSeeOther)
			return
		}
		flashSuccess(w, "Хранилище удалено")
		h.log(r, currentUser.ID, "storage.delete", "storage", storageID, nil)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
func (h *Handler) StorageSettingsPage(w http.ResponseWriter, r *http.Request) {
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

	storageDTO, err := h.storages.Get(r.Context(), currentUser.ID, storageID)
	if err != nil {
		h.render(w, r, "storage_settings.html", PageData{Title: "Settings", User: currentUser, Error: "Не удалось загрузить хранилище"})
		return
	}

	h.render(w, r, "storage_settings.html", PageData{
		Title:   "Settings",
		User:    currentUser,
		Storage: storageDTO,
	})
}

func (h *Handler) UpdateStorageSettings(w http.ResponseWriter, r *http.Request) {
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
		flashError(w, "Не удалось прочитать настройки")
		http.Redirect(w, r, "/storages/"+strconv.FormatInt(storageID, 10)+"/settings", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	visibility := domain.StorageVisibility(r.FormValue("visibility"))
	maxFileSize, _ := strconv.ParseInt(r.FormValue("maxFileSize"), 10, 64)
	maxStorageSize, _ := strconv.ParseInt(r.FormValue("maxStorageSize"), 10, 64)
	allowedFileTypes := splitMimeTypes(r.FormValue("allowedFileTypes"))
	blockedFileTypes := splitMimeTypes(r.FormValue("blockedFileTypes"))

	_, err = h.storages.Update(r.Context(), currentUser.ID, storageID, storage.UpdateStorageDTO{
		Name:             &name,
		Visibility:       &visibility,
		MaxFileSize:      &maxFileSize,
		MaxStorageSize:   &maxStorageSize,
		AllowedFileTypes: &allowedFileTypes,
		BlockedFileTypes: &blockedFileTypes,
	})
	if err != nil {
		storageDTO, _ := h.storages.Get(r.Context(), currentUser.ID, storageID)
		h.render(w, r, "storage_settings.html", PageData{
			Title:   "Settings",
			User:    currentUser,
			Storage: storageDTO,
			Error:   "Не удалось обновить хранилище",
			Flash:   &Flash{Kind: "error", Message: "Не удалось сохранить настройки"},
		})
		return
	}

	flashSuccess(w, "Настройки сохранены")
	h.log(r, currentUser.ID, "storage.update", "storage", storageID, nil)
	http.Redirect(w, r, "/storages/"+strconv.FormatInt(storageID, 10), http.StatusSeeOther)
}
