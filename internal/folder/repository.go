package folder

import (
	"context"

	"file-storage-server/internal/domain"
)

type Repository interface {
	Create(ctx context.Context, params CreateFolderParams) (*domain.Folder, error)
	GetByID(ctx context.Context, id int64) (*domain.Folder, error)
	ListByStorage(ctx context.Context, filter ListFoldersFilter) ([]domain.Folder, error)
	Rename(ctx context.Context, params RenameFolderParams) (*domain.Folder, error)
	SoftDeleteTree(ctx context.Context, id int64) (*DeleteFolderTreeResult, error)
}
