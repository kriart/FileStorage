package access

import (
	"context"

	"file-storage-server/internal/domain"
)

type Repository interface {
	UpsertStorageAccess(ctx context.Context, params UpsertStorageAccessParams) (*domain.StorageAccess, error)
	GetStorageAccess(ctx context.Context, storageID int64, userID int64) (*domain.StorageAccess, error)
	ListStorageAccesses(ctx context.Context, storageID int64) ([]domain.StorageAccess, error)
	DeleteStorageAccess(ctx context.Context, storageID int64, userID int64) error
	UpsertFolderAccess(ctx context.Context, params UpsertFolderAccessParams) (*domain.FolderAccess, error)
	GetFolderAccess(ctx context.Context, folderID int64, userID int64) (*domain.FolderAccess, error)
	ListFolderAccesses(ctx context.Context, folderID int64) ([]domain.FolderAccess, error)
	DeleteFolderAccess(ctx context.Context, folderID int64, userID int64) error
}
