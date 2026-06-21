package filesystem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type LocalStorage struct {
	rootDir string
}

func NewLocalStorage(rootDir string) *LocalStorage {
	return &LocalStorage{rootDir: rootDir}
}

func (s *LocalStorage) Save(ctx context.Context, params SaveFileParams) (*SavedFile, error) {
	staged, err := s.Stage(ctx, params)
	if err != nil {
		return nil, err
	}
	committed, err := s.Commit(ctx, staged)
	if err != nil {
		_ = s.DeleteStaged(context.Background(), staged)
		return nil, err
	}
	return committed, nil
}

func (s *LocalStorage) Stage(ctx context.Context, params SaveFileParams) (*StagedFile, error) {
	if params.Reader == nil {
		return nil, errors.New("reader is required")
	}
	if !isSafeStoredName(params.StoredName) {
		return nil, errors.New("stored name is unsafe")
	}

	relativePath := buildRelativePath(params.StoredName)
	tmpDir, err := s.safePath("tmp")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("create tmp dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(tmpDir, "upload-*")
	if err != nil {
		return nil, fmt.Errorf("create tmp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	hasher := sha256.New()
	written, copyErr := io.Copy(tmpFile, io.TeeReader(contextReader{ctx: ctx, reader: params.Reader}, hasher))
	closeErr := tmpFile.Close()
	if copyErr != nil {
		return nil, fmt.Errorf("write tmp file: %w", copyErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close tmp file: %w", closeErr)
	}

	cleanupTmp = false

	return &StagedFile{
		StoredName:   params.StoredName,
		RelativePath: relativePath,
		TempPath:     tmpPath,
		Size:         written,
		Checksum:     hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

func (s *LocalStorage) Commit(ctx context.Context, staged *StagedFile) (*SavedFile, error) {
	if staged == nil {
		return nil, errors.New("staged file is required")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	finalPath, err := s.safePath(staged.RelativePath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		return nil, fmt.Errorf("create file dir: %w", err)
	}
	if err := os.Rename(staged.TempPath, finalPath); err != nil {
		return nil, fmt.Errorf("move file into storage: %w", err)
	}
	return &SavedFile{
		StoredName:   staged.StoredName,
		RelativePath: staged.RelativePath,
		Size:         staged.Size,
		Checksum:     staged.Checksum,
	}, nil
}

func (s *LocalStorage) DeleteStaged(ctx context.Context, staged *StagedFile) error {
	if staged == nil || staged.TempPath == "" {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if err := os.Remove(staged.TempPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *LocalStorage) Open(ctx context.Context, relativePath string) (io.ReadCloser, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	path, err := s.safePath(relativePath)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *LocalStorage) Delete(ctx context.Context, relativePath string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	path, err := s.safePath(relativePath)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *LocalStorage) safePath(relativePath string) (string, error) {
	clean := filepath.Clean(relativePath)
	if filepath.IsAbs(clean) || clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", errors.New("relative path is unsafe")
	}

	root, err := filepath.Abs(s.rootDir)
	if err != nil {
		return "", err
	}

	path := filepath.Join(root, clean)
	if !strings.HasPrefix(path, root+string(filepath.Separator)) && path != root {
		return "", errors.New("relative path escapes storage root")
	}
	return path, nil
}

func buildRelativePath(storedName string) string {
	prefix := strings.ReplaceAll(storedName, "-", "")
	if len(prefix) < 4 {
		return filepath.Join("files", storedName)
	}
	return filepath.Join("files", prefix[:2], prefix[2:4], storedName)
}

func isSafeStoredName(storedName string) bool {
	if storedName == "" || strings.ContainsAny(storedName, `/\`) {
		return false
	}
	return storedName == filepath.Base(storedName)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(p)
	}
}
