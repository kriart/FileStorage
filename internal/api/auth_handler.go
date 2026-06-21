package api

import (
	"net/http"
	"time"

	"file-storage-server/internal/audit"
	"file-storage-server/internal/auth"
	"file-storage-server/internal/httpx"
	"file-storage-server/internal/middleware"
	"file-storage-server/internal/repository"
	"file-storage-server/internal/user"
)

type AuthHandler struct {
	auth  *auth.Service
	audit *audit.Service
}

func NewAuthHandler(authService *auth.Service, auditService *audit.Service) *AuthHandler {
	return &AuthHandler{auth: authService, audit: auditService}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	dto := user.RegisterUserDTO{}
	if err := httpx.DecodeJSON(r, &dto); err != nil {
		httpx.WriteError(w, err)
		return
	}

	response, err := h.auth.Register(r.Context(), dto)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	setAuthCookies(w, response)
	h.log(r, response.User.ID, "auth.register", "user", response.User.ID, nil)
	httpx.WriteJSON(w, http.StatusCreated, response)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	dto := user.LoginUserDTO{}
	if err := httpx.DecodeJSON(r, &dto); err != nil {
		httpx.WriteError(w, err)
		return
	}

	response, err := h.auth.Login(r.Context(), dto)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	setAuthCookies(w, response)
	h.log(r, response.User.ID, "auth.login", "user", response.User.ID, nil)
	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	refreshToken := ""
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		refreshToken = cookie.Value
	}
	response, err := h.auth.Refresh(r.Context(), refreshToken)
	if err != nil {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}
	setAuthCookies(w, response)
	h.log(r, response.User.ID, "auth.refresh", "user", response.User.ID, nil)
	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var userID *int64
	if currentUser, ok := middleware.CurrentUser(r.Context()); ok {
		userID = &currentUser.ID
	}
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		_ = h.auth.RevokeRefreshToken(r.Context(), cookie.Value)
	}
	clearAuthCookies(w)
	h.logPtr(r, userID, "auth.logout", "user", userID, nil)
	httpx.WriteNoContent(w)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, currentUser)
}

func setAuthCookies(w http.ResponseWriter, response *user.AuthTokenDTO) {
	accessExpiresAt, err := time.Parse(time.RFC3339, response.AccessTokenExpiresAt)
	if err != nil {
		accessExpiresAt = time.Now().Add(15 * time.Minute)
	}
	refreshExpiresAt, err := time.Parse(time.RFC3339, response.RefreshTokenExpiresAt)
	if err != nil {
		refreshExpiresAt = time.Now().Add(30 * 24 * time.Hour)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    response.AccessToken,
		Path:     "/",
		Expires:  accessExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	if response.RefreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    response.RefreshToken,
			Path:     "/",
			Expires:  refreshExpiresAt,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func clearAuthCookies(w http.ResponseWriter) {
	for _, name := range []string{"access_token", "refresh_token"} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func (h *AuthHandler) log(r *http.Request, userID int64, action string, entityType string, entityID int64, metadata map[string]any) {
	h.logPtr(r, &userID, action, entityType, &entityID, metadata)
}

func (h *AuthHandler) logPtr(r *http.Request, userID *int64, action string, entityType string, entityID *int64, metadata map[string]any) {
	ip, userAgent := audit.RequestFields(r)
	h.audit.Log(r.Context(), audit.Event{
		UserID:     userID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Metadata:   metadata,
		IP:         ip,
		UserAgent:  userAgent,
	})
}
