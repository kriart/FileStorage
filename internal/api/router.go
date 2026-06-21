package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type RouterDeps struct {
	AuthMiddleware  func(http.Handler) http.Handler
	AuthRateLimit   func(http.Handler) http.Handler
	PublicRateLimit func(http.Handler) http.Handler
	Auth            *AuthHandler
	Storages        *StorageHandler
	Accesses        *AccessHandler
	Files           *FileHandler
	Shares          *ShareHandler
}

func RegisterRoutes(router chi.Router, deps RouterDeps) {
	router.Route("/api", func(r chi.Router) {
		r.Group(func(authRoutes chi.Router) {
			if deps.AuthRateLimit != nil {
				authRoutes.Use(deps.AuthRateLimit)
			}
			authRoutes.Post("/auth/register", deps.Auth.Register)
			authRoutes.Post("/auth/login", deps.Auth.Login)
			authRoutes.Post("/auth/refresh", deps.Auth.Refresh)
			authRoutes.Post("/auth/logout", deps.Auth.Logout)
		})

		r.Group(func(protected chi.Router) {
			protected.Use(deps.AuthMiddleware)
			protected.Get("/auth/me", deps.Auth.Me)

			protected.Route("/storages", func(storages chi.Router) {
				storages.Post("/", deps.Storages.Create)
				storages.Get("/", deps.Storages.List)
				storages.Get("/{storageId}", deps.Storages.Get)
				storages.Patch("/{storageId}", deps.Storages.Update)
				storages.Delete("/{storageId}", deps.Storages.Delete)

				storages.Get("/{storageId}/accesses", deps.Accesses.ListStorageAccesses)
				storages.Post("/{storageId}/accesses", deps.Accesses.GrantStorageAccess)
				storages.Patch("/{storageId}/accesses/{userId}", deps.Accesses.UpdateStorageAccess)
				storages.Delete("/{storageId}/accesses/{userId}", deps.Accesses.DeleteStorageAccess)

				storages.Post("/{storageId}/files", deps.Files.Upload)
				storages.Get("/{storageId}/files", deps.Files.List)
			})

			protected.Get("/files/{fileId}", deps.Files.Get)
			protected.Get("/files/{fileId}/download", deps.Files.Download)
			protected.Patch("/files/{fileId}", deps.Files.Rename)
			protected.Put("/files/{fileId}", deps.Files.Replace)
			protected.Delete("/files/{fileId}", deps.Files.Delete)
			protected.Post("/files/{fileId}/links", deps.Shares.Create)
			protected.Get("/files/{fileId}/links", deps.Shares.List)
			protected.Delete("/links/{linkId}", deps.Shares.Delete)
		})

		r.Group(func(public chi.Router) {
			if deps.PublicRateLimit != nil {
				public.Use(deps.PublicRateLimit)
			}
			public.Get("/public/share/{token}", deps.Shares.PublicDownload)
			public.Post("/public/share/{token}", deps.Shares.PublicReplace)
			public.Put("/public/share/{token}", deps.Shares.PublicReplace)
		})
	})
}
