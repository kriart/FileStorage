package web

import (
	"testing"

	"file-storage-server/internal/repository"
)

func TestDisplayFileTypePrefersExtension(t *testing.T) {
	got := displayFileType("Аннотация.docx", "application/zip")
	if got != ".docx" {
		t.Fatalf("expected .docx, got %q", got)
	}
}

func TestDisplayFileTypeFallsBackToMIME(t *testing.T) {
	got := displayFileType("README", "text/plain")
	if got != "text/plain" {
		t.Fatalf("expected text/plain, got %q", got)
	}
}

func TestUploadErrorMessageKeepsSpecificLimitMessage(t *testing.T) {
	err := repository.LimitExceeded("Файл не загружен: в хранилище недостаточно свободного места")
	got := uploadErrorMessage(err)
	if got != "Файл не загружен: в хранилище недостаточно свободного места" {
		t.Fatalf("expected specific limit message, got %q", got)
	}
}
