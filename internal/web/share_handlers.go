package web

import (
	"net/http"
	"net/url"
	"strconv"

	"file-storage-server/internal/domain"
	"file-storage-server/internal/middleware"
	"file-storage-server/internal/share"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) FileLinksPage(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	fileID, err := parseID(chi.URLParam(r, "fileId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	fileDTO, err := h.files.Get(r.Context(), currentUser.ID, fileID)
	if err != nil {
		h.render(w, r, "links.html", PageData{Title: "Links", User: currentUser, Error: "Не удалось загрузить файл"})
		return
	}
	links, _ := h.shares.List(r.Context(), currentUser.ID, fileID, publicBaseURL(r))

	h.render(w, r, "links.html", PageData{
		Title: "Links",
		User:  currentUser,
		File:  fileDTO,
		Links: links,
	})
}

func (h *Handler) CreateFileLink(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	fileID, err := parseID(chi.URLParam(r, "fileId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		flashError(w, "Не удалось прочитать форму ссылки")
		http.Redirect(w, r, "/files/"+strconv.FormatInt(fileID, 10)+"/links", http.StatusSeeOther)
		return
	}

	link, err := h.shares.Create(r.Context(), currentUser.ID, fileID, share.CreateShareLinkDTO{
		AccessType: domain.ShareAccessType(r.FormValue("accessType")),
	}, publicBaseURL(r))
	if err != nil {
		flashError(w, "Не удалось создать ссылку")
		redirectBack(w, r, "/")
		return
	}

	flashSuccess(w, "Ссылка создана")
	h.log(r, currentUser.ID, "share.create", "share_link", link.ID, map[string]any{"fileId": fileID, "accessType": link.AccessType})
	redirectBackWithQuery(w, r, "/", url.Values{
		"createdLink": []string{link.URL},
		"linkFileId":  []string{strconv.FormatInt(fileID, 10)},
	})
}

func (h *Handler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	linkID, err := parseID(chi.URLParam(r, "linkId"))
	if err == nil {
		if err := h.shares.Deactivate(r.Context(), currentUser.ID, linkID); err != nil {
			flashError(w, "Не удалось отключить ссылку")
			redirectBack(w, r, "/")
			return
		}
		flashSuccess(w, "Ссылка отключена")
		h.log(r, currentUser.ID, "share.delete", "share_link", linkID, nil)
	}
	redirectBack(w, r, "/")
}
