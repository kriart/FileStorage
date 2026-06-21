package storage

import (
	"context"

	"file-storage-server/internal/domain"
)

type Repository interface {
	Create(ctx context.Context, params CreateStorageParams) (*domain.Storage, error)
	GetByID(ctx context.Context, id int64) (*domain.Storage, error)
	ListAvailableForUser(ctx context.Context, filter ListStoragesFilter) ([]domain.Storage, error)
	Update(ctx context.Context, params UpdateStorageParams) (*domain.Storage, error)
	SoftDelete(ctx context.Context, id int64) error
	ListTypeRules(ctx context.Context, storageID int64) ([]domain.StorageTypeRule, error)
	ReplaceTypeRules(ctx context.Context, storageID int64, rules []domain.StorageTypeRule) error
	AdjustUsedSize(ctx context.Context, params AdjustStorageSizeParams) (*domain.Storage, error)
}
