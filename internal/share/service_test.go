package share

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"file-storage-server/internal/domain"
	"file-storage-server/internal/file"
	"file-storage-server/internal/repository"
)

func TestPublicReplaceDoesNotIncrementUseCountWhenReplacementFails(t *testing.T) {
	ctx := context.Background()
	links := newFakeShareRepository(domain.ShareLink{
		ID:         1,
		FileID:     10,
		TokenHash:  hashToken("write-token"),
		AccessType: domain.ShareAccessWrite,
		IsActive:   true,
	})
	service := NewService(
		links,
		fakeFileRepository{file: domain.File{ID: 10, StorageID: 20, OwnerID: 30}},
		fakeFileReplaceService{err: repository.ErrInvalidInput},
		nil,
		nil,
	)

	_, err := service.PublicReplace(ctx, "write-token", "bad.bin", errReader{})
	if !errors.Is(err, repository.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if links.incrementCount != 0 {
		t.Fatalf("expected use count not to increment, got %d", links.incrementCount)
	}
}

func TestPublicReplaceIncrementsUseCountWhenReplacementSucceeds(t *testing.T) {
	ctx := context.Background()
	links := newFakeShareRepository(domain.ShareLink{
		ID:         1,
		FileID:     10,
		TokenHash:  hashToken("write-token"),
		AccessType: domain.ShareAccessWrite,
		IsActive:   true,
	})
	service := NewService(
		links,
		fakeFileRepository{file: domain.File{ID: 10, StorageID: 20, OwnerID: 30}},
		fakeFileReplaceService{callHook: true},
		nil,
		nil,
	)

	_, err := service.PublicReplace(ctx, "write-token", "ok.txt", emptyReader{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if links.incrementCount != 1 {
		t.Fatalf("expected use count to increment once, got %d", links.incrementCount)
	}
}

func TestPublicRenameIncrementsUseCountWhenRenameSucceeds(t *testing.T) {
	ctx := context.Background()
	links := newFakeShareRepository(domain.ShareLink{
		ID:         1,
		FileID:     10,
		TokenHash:  hashToken("write-token"),
		AccessType: domain.ShareAccessWrite,
		IsActive:   true,
	})
	service := NewService(
		links,
		fakeFileRepository{file: domain.File{ID: 10, StorageID: 20, OwnerID: 30, OriginalName: "old.txt"}},
		fakeFileReplaceService{callHook: true},
		nil,
		nil,
	)

	renamed, err := service.PublicRename(ctx, "write-token", "new.txt")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if renamed.OriginalName != "new.txt" {
		t.Fatalf("expected renamed file, got %q", renamed.OriginalName)
	}
	if links.incrementCount != 1 {
		t.Fatalf("expected use count to increment once, got %d", links.incrementCount)
	}
}

type fakeFileReplaceService struct {
	callHook bool
	err      error
}

func (s fakeFileReplaceService) ReplaceFromShareWithHook(
	ctx context.Context,
	fileID int64,
	originalName string,
	reader io.Reader,
	beforeCommit func(context.Context) error,
) (*file.FileDTO, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.callHook && beforeCommit != nil {
		if err := beforeCommit(ctx); err != nil {
			return nil, err
		}
	}
	return &file.FileDTO{ID: fileID, OriginalName: originalName}, nil
}

func (s fakeFileReplaceService) RenameFromShareWithHook(
	ctx context.Context,
	fileID int64,
	name string,
	beforeCommit func(context.Context) error,
) (*file.FileDTO, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.callHook && beforeCommit != nil {
		if err := beforeCommit(ctx); err != nil {
			return nil, err
		}
	}
	return &file.FileDTO{ID: fileID, OriginalName: name}, nil
}

func (s fakeFileReplaceService) DeleteFromShareWithHook(
	ctx context.Context,
	fileID int64,
	beforeCommit func(context.Context) error,
) error {
	if s.err != nil {
		return s.err
	}
	if s.callHook && beforeCommit != nil {
		return beforeCommit(ctx)
	}
	return nil
}

type fakeShareRepository struct {
	link           domain.ShareLink
	incrementCount int
}

func newFakeShareRepository(link domain.ShareLink) *fakeShareRepository {
	return &fakeShareRepository{link: link}
}

func (r *fakeShareRepository) Create(context.Context, CreateShareLinkParams) (*domain.ShareLink, error) {
	return nil, nil
}

func (r *fakeShareRepository) GetByID(context.Context, int64) (*domain.ShareLink, error) {
	return &r.link, nil
}

func (r *fakeShareRepository) GetByTokenHash(_ context.Context, tokenHash string) (*domain.ShareLink, error) {
	if tokenHash != r.link.TokenHash {
		return nil, repository.ErrNotFound
	}
	return &r.link, nil
}

func (r *fakeShareRepository) ListByFile(context.Context, ListShareLinksFilter) ([]domain.ShareLink, error) {
	return []domain.ShareLink{r.link}, nil
}

func (r *fakeShareRepository) UpdateToken(_ context.Context, _ int64, token string, tokenHash string) (*domain.ShareLink, error) {
	r.link.Token = &token
	r.link.TokenHash = tokenHash
	return &r.link, nil
}

func (r *fakeShareRepository) IncrementUseCount(context.Context, int64) error {
	r.incrementCount++
	r.link.UseCount++
	return nil
}

func (r *fakeShareRepository) Deactivate(context.Context, int64) error {
	r.link.IsActive = false
	return nil
}

type fakeFileRepository struct {
	file domain.File
}

func (r fakeFileRepository) Create(context.Context, file.CreateFileParams) (*domain.File, error) {
	return nil, nil
}

func (r fakeFileRepository) GetByID(context.Context, int64) (*domain.File, error) {
	return &r.file, nil
}

func (r fakeFileRepository) ListByStorage(context.Context, file.ListFilesFilter) ([]domain.File, error) {
	return []domain.File{r.file}, nil
}

func (r fakeFileRepository) CountByStorage(context.Context, file.ListFilesFilter) (int, error) {
	return 1, nil
}

func (r fakeFileRepository) UpdateContent(context.Context, file.UpdateFileContentParams) (*domain.File, error) {
	return &r.file, nil
}

func (r fakeFileRepository) Rename(_ context.Context, params file.RenameFileParams) (*domain.File, error) {
	r.file.OriginalName = params.OriginalName
	return &r.file, nil
}

func (r fakeFileRepository) SoftDelete(context.Context, file.SoftDeleteFileParams) (*domain.File, error) {
	deletedAt := time.Now()
	r.file.DeletedAt = &deletedAt
	return &r.file, nil
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, repository.ErrInvalidInput
}

type emptyReader struct{}

func (emptyReader) Read([]byte) (int, error) {
	return 0, io.EOF
}
