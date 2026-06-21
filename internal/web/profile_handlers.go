package web

import (
	"net/http"

	"file-storage-server/internal/middleware"
	"file-storage-server/internal/profile"
	"file-storage-server/internal/repository"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		flashError(w, "Не удалось прочитать форму профиля")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}
	parsedBirthDate, err := parseDateInput(r.FormValue("dateOfBirth"))
	if err != nil {
		flashError(w, "Некорректная дата рождения")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}
	updated, err := h.profile.UpdateSettings(r.Context(), currentUser.ID, profile.UpdateSettings{
		Username:    r.FormValue("username"),
		DateOfBirth: parsedBirthDate,
	})
	if err != nil {
		flashError(w, repository.PublicMessage(err))
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}
	flashSuccess(w, "Профиль обновлен")
	h.log(r, currentUser.ID, "profile.update", "user", currentUser.ID, map[string]any{
		"username":    updated.Username,
		"dateOfBirth": updated.DateOfBirth,
	})
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

func (h *Handler) UpdateProfilePreferences(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		flashError(w, "Не удалось прочитать настройки")
		redirectBack(w, r, "/profile")
		return
	}
	theme := currentUser.Theme
	if r.FormValue("theme") != "" {
		theme = r.FormValue("theme")
	}
	language := currentUser.Language
	if r.FormValue("language") != "" {
		language = r.FormValue("language")
	}
	updated, err := h.profile.UpdatePreferences(r.Context(), currentUser.ID, profile.UpdatePreferences{
		Theme:    theme,
		Language: language,
	})
	if err != nil {
		flashError(w, "Не удалось обновить настройки")
		redirectBack(w, r, "/profile")
		return
	}
	h.log(r, currentUser.ID, "profile.preferences_update", "user", currentUser.ID, map[string]any{
		"theme":    updated.Theme,
		"language": updated.Language,
	})
	http.Redirect(w, r, safeReturnTo(r.FormValue("returnTo"), "/profile"), http.StatusSeeOther)
}

func (h *Handler) UpdateProfileAvatar(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	fileHeader, err := uploadedAvatar(r)
	if err != nil {
		flashError(w, err.Error())
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	relativePath, err := h.saveAvatar(currentUser.ID, fileHeader)
	if err != nil {
		flashError(w, "Не удалось сохранить фото")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	previousPath := currentUser.AvatarPath
	if _, err := h.profile.UpdateAvatarPath(r.Context(), currentUser.ID, relativePath); err != nil {
		_ = h.removeStoredAvatar(relativePath)
		flashError(w, "Не удалось обновить фото профиля")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}
	if previousPath != nil {
		_ = h.removeStoredAvatar(*previousPath)
	}

	flashSuccess(w, "Фото профиля обновлено")
	h.log(r, currentUser.ID, "profile.avatar_update", "user", currentUser.ID, nil)
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

func (h *Handler) ProfileAvatar(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if currentUser.AvatarPath == nil || *currentUser.AvatarPath == "" {
		http.NotFound(w, r)
		return
	}
	fullPath, ok := h.avatarPath(*currentUser.AvatarPath)
	if !ok {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, fullPath)
}

func (h *Handler) RevokeProfileSession(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	sessionID, err := parseID(chi.URLParam(r, "sessionId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := h.profile.RevokeSession(r.Context(), currentUser.ID, sessionID); err != nil {
		flashError(w, "Не удалось завершить сессию")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}
	flashSuccess(w, "Сессия завершена")
	h.log(r, currentUser.ID, "profile.session_revoke", "refresh_token", sessionID, nil)
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}
