package web

import "net/http"

func (h *Handler) render(w http.ResponseWriter, r *http.Request, name string, data PageData) {
	if data.User == nil {
		data.User = currentUser(r)
	}
	if data.Flash == nil {
		data.Flash = readFlash(w, r)
	}
	data.CurrentURL = r.URL.RequestURI()
	data.Theme = "light"
	data.Language = "ru"
	if data.User != nil {
		if data.User.Theme == "dark" {
			data.Theme = "dark"
		}
		if data.User.Language == "en" {
			data.Language = "en"
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
