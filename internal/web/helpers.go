package web

import (
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"file-storage-server/internal/auth"
	"file-storage-server/internal/domain"
	"file-storage-server/internal/middleware"
	"file-storage-server/internal/repository"
)

func uploadErrorMessage(err error) string {
	publicMessage := repository.PublicMessage(err)
	switch {
	case errors.Is(err, repository.ErrInvalidInput):
		if publicMessage != "Некорректные данные" {
			return publicMessage
		}
		return "Файл не загружен: тип или данные файла не подходят"
	case errors.Is(err, repository.ErrLimitExceeded):
		if publicMessage != "Превышен лимит" {
			return publicMessage
		}
		return "Файл не загружен: превышен лимит размера"
	case errors.Is(err, repository.ErrForbidden):
		return "Недостаточно прав для этого действия"
	default:
		return "Не удалось обработать файл"
	}
}

func currentUser(r *http.Request) *auth.CurrentUser {
	currentUser, _ := middleware.CurrentUser(r.Context())
	return currentUser
}
func parseID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, repository.ErrInvalidInput
	}
	return id, nil
}

func parsePage(value string) int {
	page, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func parseOptionalID(value string) (*int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	id, err := parseID(value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func splitMimeTypes(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			result = append(result, field)
		}
	}
	return result
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

func redirectBack(w http.ResponseWriter, r *http.Request, fallback string) {
	location := r.Header.Get("Referer")
	if location == "" {
		location = fallback
	}
	http.Redirect(w, r, location, http.StatusSeeOther)
}

func safeReturnTo(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return fallback
	}
	return value
}

func redirectBackWithQuery(w http.ResponseWriter, r *http.Request, fallback string, values url.Values) {
	location := r.Header.Get("Referer")
	if location == "" {
		location = fallback
	}

	parsed, err := url.Parse(location)
	if err != nil {
		http.Redirect(w, r, fallback, http.StatusSeeOther)
		return
	}
	query := parsed.Query()
	for key, value := range values {
		query.Del(key)
		for _, item := range value {
			query.Add(key, item)
		}
	}
	parsed.RawQuery = query.Encode()
	http.Redirect(w, r, parsed.String(), http.StatusSeeOther)
}

func redirectToStorageFolder(w http.ResponseWriter, r *http.Request, storageID int64, folderID *int64) {
	location := "/storages/" + strconv.FormatInt(storageID, 10)
	if folderID != nil {
		location += "?folderId=" + strconv.FormatInt(*folderID, 10)
	}
	http.Redirect(w, r, location, http.StatusSeeOther)
}

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

func parseDateInput(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}
	today := time.Now().In(time.Local)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	if parsed.After(today) {
		return nil, repository.ErrInvalidInput
	}
	return &parsed, nil
}

func formatDateInput(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02")
}

func formatDateDisplay(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("02.01.2006")
}

func formatDuration(seconds float64) string {
	switch {
	case seconds < 0.001:
		return strconv.FormatFloat(seconds*1000000, 'f', 0, 64) + " us"
	case seconds < 1:
		return strconv.FormatFloat(seconds*1000, 'f', 1, 64) + " ms"
	default:
		return strconv.FormatFloat(seconds, 'f', 2, 64) + " s"
	}
}

func formatSeconds(seconds int64) string {
	duration := time.Duration(seconds) * time.Second
	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour
	hours := duration / time.Hour
	duration -= hours * time.Hour
	minutes := duration / time.Minute
	if days > 0 {
		return strconv.FormatInt(int64(days), 10) + " д " + strconv.FormatInt(int64(hours), 10) + " ч"
	}
	if hours > 0 {
		return strconv.FormatInt(int64(hours), 10) + " ч " + strconv.FormatInt(int64(minutes), 10) + " мин"
	}
	return strconv.FormatInt(int64(minutes), 10) + " мин"
}
func usagePercent(usedSize, maxSize int64) int {
	if usedSize <= 0 || maxSize <= 0 {
		return 0
	}
	percent := int((usedSize * 100) / maxSize)
	if percent < 1 {
		return 1
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func displayAccess(level domain.StorageAccessLevel) string {
	return displayAccessLang("ru", level)
}

func displayAccessLang(language string, level domain.StorageAccessLevel) string {
	switch level {
	case domain.StorageAccessViewer:
		return translate(language, "access.viewer")
	case domain.StorageAccessUploader:
		return translate(language, "access.uploader")
	case domain.StorageAccessManager:
		return translate(language, "access.manager")
	case domain.StorageAccessOwner:
		return translate(language, "access.owner")
	default:
		return string(level)
	}
}

func displayShare(accessType domain.ShareAccessType) string {
	return displayShareLang("ru", accessType)
}

func displayShareLang(language string, accessType domain.ShareAccessType) string {
	switch accessType {
	case domain.ShareAccessRead:
		return translate(language, "share.read")
	case domain.ShareAccessWrite:
		return translate(language, "share.write")
	default:
		return string(accessType)
	}
}

func displayVisibility(visibility domain.StorageVisibility) string {
	return displayVisibilityLang("ru", visibility)
}

func displayVisibilityLang(language string, visibility domain.StorageVisibility) string {
	switch visibility {
	case domain.StorageVisibilityPrivate:
		return translate(language, "visibility.private")
	case domain.StorageVisibilityPublicRead:
		return translate(language, "visibility.public_read")
	case domain.StorageVisibilityPublicUpload:
		return translate(language, "visibility.public_upload")
	default:
		return string(visibility)
	}
}

func displayStorageType(storageType domain.StorageType) string {
	return displayStorageTypeLang("ru", storageType)
}

func displayStorageTypeLang(language string, storageType domain.StorageType) string {
	switch storageType {
	case domain.StorageTypePersonal:
		return translate(language, "storage.type.personal")
	case domain.StorageTypeGlobal:
		return translate(language, "storage.type.global")
	default:
		return string(storageType)
	}
}

func displayFileType(originalName string, mimeType string) string {
	return displayFileTypeLang("ru", originalName, mimeType)
}

func displayFileTypeLang(language string, originalName string, mimeType string) string {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(originalName)))
	if ext != "" {
		return ext
	}
	if mimeType != "" {
		return mimeType
	}
	return translate(language, "storage.file")
}
