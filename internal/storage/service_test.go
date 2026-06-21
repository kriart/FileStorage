package storage

import (
	"reflect"
	"testing"
)

func TestNormalizeMimeTypesAcceptsArbitraryExtensionsAndMIME(t *testing.T) {
	got := normalizeMimeTypes([]string{
		"txt",
		".PDF",
		"*.custom-format",
		"image/png",
		"txt",
		" report.docx ",
	})
	want := []string{".txt", ".pdf", ".custom-format", "image/png", ".docx"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
