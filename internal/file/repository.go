package file

import (
	"context"

	"file-storage-server/internal/domain"
)

type Repository interface {
	Create(ctx context.Context, params CreateFileParams) (*domain.File, error)
	GetByID(ctx context.Context, id int64) (*domain.File, error)
	ListByStorage(ctx context.Context, filter ListFilesFilter) ([]domain.File, error)
	CountByStorage(ctx context.Context, filter ListFilesFilter) (int, error)
	UpdateContent(ctx context.Context, params UpdateFileContentParams) (*domain.File, error)
	Rename(ctx context.Context, params RenameFileParams) (*domain.File, error)
	SoftDelete(ctx context.Context, params SoftDeleteFileParams) (*domain.File, error)
}
