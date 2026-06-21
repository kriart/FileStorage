package filesystem

import (
	"context"
	"io"
)

type SaveFileParams struct {
	Reader      io.Reader
	StoredName  string
	ContentType string
}

type SavedFile struct {
	StoredName   string
	RelativePath string
	Size         int64
	Checksum     string
}

type StagedFile struct {
	StoredName   string
	RelativePath string
	TempPath     string
	Size         int64
	Checksum     string
}

type Storage interface {
	Save(ctx context.Context, params SaveFileParams) (*SavedFile, error)
	Stage(ctx context.Context, params SaveFileParams) (*StagedFile, error)
	Commit(ctx context.Context, staged *StagedFile) (*SavedFile, error)
	DeleteStaged(ctx context.Context, staged *StagedFile) error
	Open(ctx context.Context, relativePath string) (io.ReadCloser, error)
	Delete(ctx context.Context, relativePath string) error
}
