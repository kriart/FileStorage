package web

import (
	"net/http"
	"time"

	"file-storage-server/internal/user"
)

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
