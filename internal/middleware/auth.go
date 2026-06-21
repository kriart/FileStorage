package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"file-storage-server/internal/auth"
)

type currentUserContextKey struct{}

func Auth(authService *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r.Header.Get("Authorization"))
			if token == "" {
				cookie, err := r.Cookie("access_token")
				if err == nil {
					token = cookie.Value
				}
			}

			currentUser, err := authService.CurrentUser(r.Context(), token)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), currentUserContextKey{}, currentUser)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func WebAuth(authService *auth.Service, loginPath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r.Header.Get("Authorization"))
			if token == "" {
				cookie, err := r.Cookie("access_token")
				if err == nil {
					token = cookie.Value
				}
			}

			currentUser, err := authService.CurrentUser(r.Context(), token)
			if err != nil {
				if cookie, cookieErr := r.Cookie("refresh_token"); cookieErr == nil {
					response, refreshErr := authService.Refresh(r.Context(), cookie.Value)
					if refreshErr == nil {
						setAuthCookies(w, response.AccessToken, response.RefreshToken, response.AccessTokenExpiresAt, response.RefreshTokenExpiresAt)
						currentUser, err = authService.CurrentUser(r.Context(), response.AccessToken)
					}
				}
				if err != nil {
					clearAuthCookies(w)
					http.Redirect(w, r, loginPath, http.StatusSeeOther)
					return
				}
			}

			ctx := context.WithValue(r.Context(), currentUserContextKey{}, currentUser)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func setAuthCookies(w http.ResponseWriter, accessToken string, refreshToken string, accessExpiresAtValue string, refreshExpiresAtValue string) {
	accessExpiresAt, err := time.Parse(time.RFC3339, accessExpiresAtValue)
	if err != nil {
		accessExpiresAt = time.Now().Add(15 * time.Minute)
	}
	refreshExpiresAt, err := time.Parse(time.RFC3339, refreshExpiresAtValue)
	if err != nil {
		refreshExpiresAt = time.Now().Add(30 * 24 * time.Hour)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		Expires:  accessExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	if refreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
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

func CurrentUser(ctx context.Context) (*auth.CurrentUser, bool) {
	currentUser, ok := ctx.Value(currentUserContextKey{}).(*auth.CurrentUser)
	return currentUser, ok
}

func bearerToken(header string) string {
	if header == "" {
		return ""
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}
