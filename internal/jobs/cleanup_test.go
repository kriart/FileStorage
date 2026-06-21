package jobs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPruneEmptyParentsStopsAtStorageArea(t *testing.T) {
	root := t.TempDir()
	filesDir := filepath.Join(root, "files")
	leafDir := filepath.Join(filesDir, "ab", "cd")
	if err := os.MkdirAll(leafDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := pruneEmptyParents(leafDir, filesDir); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(leafDir); !os.IsNotExist(err) {
		t.Fatalf("leaf dir should be removed, stat err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filesDir, "ab")); !os.IsNotExist(err) {
		t.Fatalf("empty shard dir should be removed, stat err: %v", err)
	}
	if _, err := os.Stat(filesDir); err != nil {
		t.Fatalf("files root should remain: %v", err)
	}
}

func TestPruneEmptyParentsKeepsNonEmptyParent(t *testing.T) {
	root := t.TempDir()
	filesDir := filepath.Join(root, "files")
	leafDir := filepath.Join(filesDir, "ab", "cd")
	siblingDir := filepath.Join(filesDir, "ab", "ef")
	if err := os.MkdirAll(leafDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(siblingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siblingDir, "active.txt"), []byte("active"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := pruneEmptyParents(leafDir, filesDir); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(leafDir); !os.IsNotExist(err) {
		t.Fatalf("leaf dir should be removed, stat err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filesDir, "ab")); err != nil {
		t.Fatalf("non-empty shard dir should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(siblingDir, "active.txt")); err != nil {
		t.Fatalf("sibling file should remain: %v", err)
	}
}

func TestPruneEmptyParentsIgnoresOutsideStart(t *testing.T) {
	root := t.TempDir()
	filesDir := filepath.Join(root, "files")
	outsideDir := filepath.Join(root, "outside", "aa")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := pruneEmptyParents(outsideDir, filesDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outsideDir); err != nil {
		t.Fatalf("outside dir should not be touched: %v", err)
	}
}
