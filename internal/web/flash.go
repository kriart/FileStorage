package web

import (
	"net/http"
	"net/url"
	"strings"
)

func flashSuccess(w http.ResponseWriter, message string) {
	setFlash(w, "success", message)
}

func flashError(w http.ResponseWriter, message string) {
	setFlash(w, "error", message)
}

func setFlash(w http.ResponseWriter, kind string, message string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "flash",
		Value:    url.QueryEscape(kind + "|" + message),
		Path:     "/",
		MaxAge:   30,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func readFlash(w http.ResponseWriter, r *http.Request) *Flash {
	cookie, err := r.Cookie("flash")
	if err != nil || cookie.Value == "" {
		return nil
	}
	clearFlash(w)

	value, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return nil
	}
	kind, message, ok := strings.Cut(value, "|")
	if !ok || message == "" {
		return nil
	}
	if kind != "success" && kind != "error" {
		kind = "success"
	}
	return &Flash{Kind: kind, Message: message}
}

func clearFlash(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "flash",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
