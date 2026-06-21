package web

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

func uploadedAvatar(r *http.Request) (*multipart.FileHeader, error) {
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		return nil, errors.New("Не удалось прочитать фото")
	}
	opened, header, err := r.FormFile("avatar")
	if err != nil {
		return nil, errors.New("Выберите фото")
	}
	if opened != nil {
		_ = opened.Close()
	}
	if header.Size <= 0 {
		return nil, errors.New("Фото пустое")
	}
	if header.Size > 5<<20 {
		return nil, errors.New("Фото должно быть не больше 5 MiB")
	}
	return header, nil
}

func (h *Handler) saveAvatar(userID int64, header *multipart.FileHeader) (string, error) {
	src, err := header.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	buffer := make([]byte, 512)
	n, err := src.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	contentType := http.DetectContentType(buffer[:n])
	ext := avatarExtension(contentType)
	if ext == "" {
		return "", errors.New("Можно загрузить только PNG, JPG или WebP")
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	userDir := filepath.Join(h.fileRoot, "avatars", strconv.FormatInt(userID, 10))
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return "", err
	}

	filename := uuid.NewString() + ext
	fullPath := filepath.Join(userDir, filename)
	dst, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(fullPath)
		return "", err
	}
	return filepath.ToSlash(filepath.Join("avatars", strconv.FormatInt(userID, 10), filename)), nil
}

func avatarExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}

func (h *Handler) avatarPath(relativePath string) (string, bool) {
	clean := strings.TrimPrefix(filepath.Clean("/"+relativePath), string(filepath.Separator))
	if clean == "." || !strings.HasPrefix(filepath.ToSlash(clean), "avatars/") {
		return "", false
	}
	fullPath := filepath.Join(h.fileRoot, clean)
	root, err := filepath.Abs(filepath.Join(h.fileRoot, "avatars"))
	if err != nil {
		return "", false
	}
	full, err := filepath.Abs(fullPath)
	if err != nil {
		return "", false
	}
	if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return "", false
	}
	return full, true
}

func (h *Handler) removeStoredAvatar(relativePath string) error {
	fullPath, ok := h.avatarPath(relativePath)
	if !ok {
		return nil
	}
	return os.Remove(fullPath)
}
