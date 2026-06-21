package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) Routes(router chi.Router, authMiddleware func(http.Handler) http.Handler, authRateLimit func(http.Handler) http.Handler) {
	router.Get("/login", h.LoginPage)
	router.Get("/register", h.RegisterPage)
	if authRateLimit == nil {
		router.Post("/login", h.Login)
		router.Post("/register", h.Register)
		router.Post("/logout", h.Logout)
	} else {
		router.With(authRateLimit).Post("/login", h.Login)
		router.With(authRateLimit).Post("/register", h.Register)
		router.With(authRateLimit).Post("/logout", h.Logout)
	}

	router.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/", h.StorageIndex)
		r.Get("/admin", h.AdminPage)
		r.Get("/metrics", h.MetricsPage)
		r.Get("/profile", h.ProfilePage)
		r.Post("/profile", h.UpdateProfile)
		r.Post("/profile/preferences", h.UpdateProfilePreferences)
		r.Get("/profile/avatar", h.ProfileAvatar)
		r.Post("/profile/avatar", h.UpdateProfileAvatar)
		r.Post("/profile/sessions/{sessionId}/revoke", h.RevokeProfileSession)
		r.Get("/storages/new", h.NewStoragePage)
		r.Post("/storages", h.CreateStorage)
		r.Get("/storages/{storageId}", h.StorageDetail)
		r.Get("/storages/{storageId}/settings", h.StorageSettingsPage)
		r.Post("/storages/{storageId}/settings", h.UpdateStorageSettings)
		r.Post("/storages/{storageId}/delete", h.DeleteStorage)
		r.Post("/storages/{storageId}/folders", h.CreateFolder)
		r.Post("/storages/{storageId}/files", h.UploadFile)
		r.Post("/storages/{storageId}/accesses", h.GrantAccess)
		r.Post("/storages/{storageId}/accesses/{userId}/delete", h.DeleteAccess)
		r.Post("/folders/{folderId}/accesses", h.GrantFolderAccess)
		r.Post("/folders/{folderId}/accesses/{userId}/delete", h.DeleteFolderAccess)
		r.Post("/folders/{folderId}/rename", h.RenameFolder)
		r.Post("/folders/{folderId}/delete", h.DeleteFolder)
		r.Post("/files/{fileId}/rename", h.RenameFile)
		r.Post("/files/{fileId}/replace", h.ReplaceFile)
		r.Post("/files/{fileId}/delete", h.DeleteFile)
		r.Get("/files/{fileId}/links", h.FileLinksPage)
		r.Post("/files/{fileId}/links", h.CreateFileLink)
		r.Post("/links/{linkId}/delete", h.DeleteLink)
	})
}
