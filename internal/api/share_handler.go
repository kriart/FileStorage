package api

import (
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"

	"file-storage-server/internal/audit"
	"file-storage-server/internal/domain"
	"file-storage-server/internal/httpx"
	"file-storage-server/internal/middleware"
	"file-storage-server/internal/repository"
	"file-storage-server/internal/share"

	"github.com/go-chi/chi/v5"
)

type ShareHandler struct {
	links *share.Service
	audit *audit.Service
}

func NewShareHandler(links *share.Service, auditService *audit.Service) *ShareHandler {
	return &ShareHandler{links: links, audit: auditService}
}

func (h *ShareHandler) Create(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	fileID, err := parseID(chi.URLParam(r, "fileId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	dto := share.CreateShareLinkDTO{}
	if err := httpx.DecodeJSON(r, &dto); err != nil {
		httpx.WriteError(w, err)
		return
	}

	response, err := h.links.Create(r.Context(), currentUser.ID, fileID, dto, publicBaseURL(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	auditLog(r, h.audit, currentUser, "share.create", "share_link", response.ID, map[string]any{"fileId": fileID, "accessType": response.AccessType})
	httpx.WriteJSON(w, http.StatusCreated, response)
}

func (h *ShareHandler) List(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	fileID, err := parseID(chi.URLParam(r, "fileId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	response, err := h.links.List(r.Context(), currentUser.ID, fileID, publicBaseURL(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *ShareHandler) Delete(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := middleware.CurrentUser(r.Context())
	if !ok {
		httpx.WriteError(w, repository.ErrUnauthorized)
		return
	}

	linkID, err := parseID(chi.URLParam(r, "linkId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	if err := h.links.Deactivate(r.Context(), currentUser.ID, linkID); err != nil {
		httpx.WriteError(w, err)
		return
	}

	auditLog(r, h.audit, currentUser, "share.delete", "share_link", linkID, nil)
	httpx.WriteNoContent(w)
}

func (h *ShareHandler) PublicDownload(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	info, err := h.links.PublicInfo(r.Context(), token)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if info.Link.AccessType == domain.ShareAccessWrite && r.URL.Query().Get("download") != "1" {
		renderPublicReplacePage(w, publicReplacePageData{
			Action:   r.URL.Path,
			FileName: info.File.OriginalName,
			FileSize: info.File.Size,
		})
		return
	}

	download, err := h.links.PublicDownload(r.Context(), token)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	defer download.Reader.Close()

	w.Header().Set("Content-Type", download.File.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(download.File.Size, 10))
	w.Header().Set("Content-Disposition", contentDisposition(download.File.OriginalName))
	w.WriteHeader(http.StatusOK)
	auditLog(r, h.audit, nil, "share.public_download", "file", download.File.ID, map[string]any{"storageId": download.File.StorageID})
	_, _ = io.Copy(w, download.Reader)
}

func (h *ShareHandler) PublicReplace(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	wantsHTML := r.Method == http.MethodPost || strings.Contains(r.Header.Get("Accept"), "text/html")

	info, err := h.links.PublicInfo(r.Context(), token)
	if err != nil {
		if wantsHTML {
			renderPublicReplacePage(w, publicReplacePageData{
				Action: r.URL.Path,
				Error:  repository.PublicMessage(err),
			})
			return
		}
		httpx.WriteError(w, err)
		return
	}

	if r.Method == http.MethodPost {
		switch r.URL.Query().Get("action") {
		case "rename":
			h.publicRename(w, r, token, info, wantsHTML)
			return
		case "delete":
			h.publicDelete(w, r, token, info, wantsHTML)
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxMultipartBodySize)
	src, header, err := r.FormFile("file")
	if err != nil {
		if wantsHTML {
			renderPublicReplacePage(w, publicReplacePageData{
				Action:   r.URL.Path,
				FileName: info.File.OriginalName,
				FileSize: info.File.Size,
				Error:    "Выберите файл для замены",
			})
			return
		}
		httpx.WriteError(w, repository.ErrInvalidInput)
		return
	}
	defer src.Close()

	response, err := h.links.PublicReplace(r.Context(), token, header.Filename, src)
	if err != nil {
		if wantsHTML {
			renderPublicReplacePage(w, publicReplacePageData{
				Action:   r.URL.Path,
				FileName: info.File.OriginalName,
				FileSize: info.File.Size,
				Error:    repository.PublicMessage(err),
			})
			return
		}
		httpx.WriteError(w, err)
		return
	}

	auditLog(r, h.audit, nil, "share.public_replace", "file", response.ID, map[string]any{"name": response.OriginalName, "size": response.Size})
	if wantsHTML {
		renderPublicReplacePage(w, publicReplacePageData{
			Action:   r.URL.Path,
			FileName: response.OriginalName,
			FileSize: response.Size,
			Success:  "Файл заменен",
		})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *ShareHandler) publicRename(w http.ResponseWriter, r *http.Request, token string, info *share.PublicLinkInfo, wantsHTML bool) {
	if err := r.ParseForm(); err != nil {
		if wantsHTML {
			renderPublicReplacePage(w, publicReplacePageData{
				Action:   r.URL.Path,
				FileName: info.File.OriginalName,
				FileSize: info.File.Size,
				Error:    "Не удалось прочитать форму",
			})
			return
		}
		httpx.WriteError(w, repository.ErrInvalidInput)
		return
	}

	response, err := h.links.PublicRename(r.Context(), token, r.FormValue("name"))
	if err != nil {
		if wantsHTML {
			renderPublicReplacePage(w, publicReplacePageData{
				Action:   r.URL.Path,
				FileName: info.File.OriginalName,
				FileSize: info.File.Size,
				Error:    repository.PublicMessage(err),
			})
			return
		}
		httpx.WriteError(w, err)
		return
	}

	auditLog(r, h.audit, nil, "share.public_rename", "file", response.ID, map[string]any{"name": response.OriginalName})
	if wantsHTML {
		renderPublicReplacePage(w, publicReplacePageData{
			Action:   r.URL.Path,
			FileName: response.OriginalName,
			FileSize: response.Size,
			Success:  "Файл переименован",
		})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *ShareHandler) publicDelete(w http.ResponseWriter, r *http.Request, token string, info *share.PublicLinkInfo, wantsHTML bool) {
	if err := h.links.PublicDelete(r.Context(), token); err != nil {
		if wantsHTML {
			renderPublicReplacePage(w, publicReplacePageData{
				Action:   r.URL.Path,
				FileName: info.File.OriginalName,
				FileSize: info.File.Size,
				Error:    repository.PublicMessage(err),
			})
			return
		}
		httpx.WriteError(w, err)
		return
	}

	auditLog(r, h.audit, nil, "share.public_delete", "file", info.File.ID, map[string]any{"name": info.File.OriginalName})
	if wantsHTML {
		renderPublicReplacePage(w, publicReplacePageData{
			Action:  r.URL.Path,
			Success: "Файл удален",
			Deleted: true,
		})
		return
	}
	httpx.WriteNoContent(w)
}

type publicReplacePageData struct {
	Action   string
	FileName string
	FileSize int64
	Error    string
	Success  string
	Deleted  bool
}

func renderPublicReplacePage(w http.ResponseWriter, data publicReplacePageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = publicReplaceTemplate.Execute(w, data)
}

var publicReplaceTemplate = template.Must(template.New("public-replace").Funcs(template.FuncMap{
	"formatBytes": formatBytes,
}).Parse(`<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Редактирование файла</title>
  <style>
    :root {
      color-scheme: light dark;
      --bg: #f3f6f8;
      --panel: #ffffff;
      --text: #17212f;
      --muted: #64748b;
      --line: #d8e1ec;
      --primary: #1f7a66;
      --primary-dark: #17624f;
      --danger-bg: #fff1f1;
      --danger: #a3261d;
      --success-bg: #eef8f3;
      --success: #14624f;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      display: grid;
      place-items: center;
      padding: 24px;
      background: var(--bg);
      color: var(--text);
      font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    main {
      width: min(560px, 100%);
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
      padding: 28px;
      box-shadow: 0 18px 50px rgba(20, 35, 55, .12);
    }
    h1 { margin: 0 0 10px; font-size: 28px; }
    p { margin: 0 0 18px; color: var(--muted); }
    .file {
      display: flex;
      justify-content: space-between;
      gap: 16px;
      padding: 14px 0;
      border-top: 1px solid var(--line);
      border-bottom: 1px solid var(--line);
      margin-bottom: 20px;
      font-weight: 700;
    }
    .file span:last-child { color: var(--muted); font-weight: 600; white-space: nowrap; }
    form { display: grid; gap: 10px; margin-top: 16px; }
    label { display: grid; gap: 8px; color: var(--muted); font-weight: 700; }
    input[type="file"], input[type="text"] {
      width: 100%;
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 12px;
      background: transparent;
      color: var(--text);
    }
    button {
      justify-self: start;
      border: 0;
      border-radius: 8px;
      padding: 12px 18px;
      background: var(--primary);
      color: white;
      font-weight: 800;
      cursor: pointer;
    }
    button:hover { background: var(--primary-dark); }
    .button-link {
      display: inline-flex;
      width: fit-content;
      border-radius: 8px;
      padding: 12px 18px;
      background: var(--primary);
      color: white;
      font-weight: 800;
      text-decoration: none;
    }
    .button-link:hover { background: var(--primary-dark); }
    .danger-button { background: #b42318; }
    .danger-button:hover { background: #8f1d15; }
    .edit-actions { margin-bottom: 6px; }
    .message {
      border-radius: 8px;
      padding: 12px 14px;
      margin-bottom: 16px;
      font-weight: 700;
    }
    .message.error { background: var(--danger-bg); color: var(--danger); }
    .message.success { background: var(--success-bg); color: var(--success); }
  </style>
</head>
<body>
  <main>
    <h1>Редактирование файла</h1>
    <p>Эта ссылка позволяет скачать, переименовать, заменить или удалить только указанный файл. Доступ к хранилищу не выдается.</p>
    {{if .Error}}<div class="message error">{{.Error}}</div>{{end}}
    {{if .Success}}<div class="message success">{{.Success}}</div>{{end}}
    {{if and .FileName (not .Deleted)}}
    <div class="file"><span>{{.FileName}}</span><span>{{formatBytes .FileSize}}</span></div>
    <div class="edit-actions">
      <a class="button-link" href="{{.Action}}?download=1">Скачать файл</a>
    </div>
    <form method="post" action="{{.Action}}?action=rename">
      <label>
        <span>Новое название</span>
        <input type="text" name="name" value="{{.FileName}}" required>
      </label>
      <button type="submit">Переименовать</button>
    </form>
    <form method="post" enctype="multipart/form-data" action="{{.Action}}?action=replace">
      <label>
        <span>Новый файл</span>
        <input type="file" name="file" required>
      </label>
      <button type="submit">Заменить файл</button>
    </form>
    <form method="post" action="{{.Action}}?action=delete" onsubmit="return confirm('Удалить файл?');">
      <button class="danger-button" type="submit">Удалить файл</button>
    </form>
    {{end}}
  </main>
</body>
</html>`))

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return strconv.FormatInt(size, 10) + " B"
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return strconv.FormatFloat(float64(size)/float64(div), 'f', 1, 64) + " " + string("KMGTPE"[exp]) + "iB"
}

func publicBaseURL(r *http.Request) string {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}

	return scheme + "://" + host
}
