package web

import (
	"net/http"

	"file-storage-server/internal/domain"
	"file-storage-server/internal/middleware"
)

func (h *Handler) StorageIndex(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	storages, err := h.storages.List(r.Context(), currentUser.ID)
	if err != nil {
		h.render(w, r, "storages.html", PageData{Title: "Storages", Error: "Не удалось загрузить хранилища"})
		return
	}

	h.render(w, r, "storages.html", PageData{
		Title:    "Storages",
		User:     currentUser,
		Storages: storages,
	})
}

func (h *Handler) AdminPage(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if currentUser.Role != domain.UserRoleAdmin {
		http.NotFound(w, r)
		return
	}

	dashboard, err := h.admin.Dashboard(r.Context())
	if err != nil {
		h.render(w, r, "admin.html", PageData{Title: "Admin", User: currentUser, Error: "Не удалось загрузить админ-панель"})
		return
	}
	h.render(w, r, "admin.html", PageData{Title: "Admin", User: currentUser, Admin: dashboard})
}

func (h *Handler) MetricsPage(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if currentUser.Role != domain.UserRoleAdmin {
		http.NotFound(w, r)
		return
	}
	h.render(w, r, "metrics.html", PageData{
		Title:   "Metrics",
		User:    currentUser,
		Metrics: h.metrics.Snapshot(),
	})
}

func (h *Handler) ProfilePage(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	profileDTO, err := h.profile.Get(r.Context(), currentUser.ID)
	if err != nil {
		h.render(w, r, "profile.html", PageData{Title: "Profile", User: currentUser, Error: "Не удалось загрузить профиль"})
		return
	}
	h.render(w, r, "profile.html", PageData{Title: "Profile", User: currentUser, Profile: profileDTO})
}
