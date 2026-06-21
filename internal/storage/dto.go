package storage

import "file-storage-server/internal/domain"

type CreateStorageDTO struct {
	Name             string                   `json:"name"`
	Type             domain.StorageType       `json:"type"`
	Visibility       domain.StorageVisibility `json:"visibility"`
	MaxFileSize      int64                    `json:"maxFileSize"`
	MaxStorageSize   int64                    `json:"maxStorageSize"`
	AllowedFileTypes []string                 `json:"allowedFileTypes"`
	BlockedFileTypes []string                 `json:"blockedFileTypes"`
}

type UpdateStorageDTO struct {
	Name             *string                   `json:"name,omitempty"`
	Visibility       *domain.StorageVisibility `json:"visibility,omitempty"`
	MaxFileSize      *int64                    `json:"maxFileSize,omitempty"`
	MaxStorageSize   *int64                    `json:"maxStorageSize,omitempty"`
	AllowedFileTypes *[]string                 `json:"allowedFileTypes,omitempty"`
	BlockedFileTypes *[]string                 `json:"blockedFileTypes,omitempty"`
}

type StorageDTO struct {
	ID               int64                    `json:"id"`
	Name             string                   `json:"name"`
	Type             domain.StorageType       `json:"type"`
	Visibility       domain.StorageVisibility `json:"visibility"`
	MaxFileSize      int64                    `json:"maxFileSize"`
	MaxStorageSize   int64                    `json:"maxStorageSize"`
	UsedSize         int64                    `json:"usedSize"`
	AllowedFileTypes []string                 `json:"allowedFileTypes"`
	BlockedFileTypes []string                 `json:"blockedFileTypes"`
}

type ListStoragesFilter struct {
	UserID         int64
	IncludeDeleted bool
	Type           *domain.StorageType
}

type CreateStorageParams struct {
	Name             string
	Type             domain.StorageType
	Visibility       domain.StorageVisibility
	MaxFileSize      int64
	MaxStorageSize   int64
	AllowedFileTypes []string
	BlockedFileTypes []string
	OwnerID          int64
}

type UpdateStorageParams struct {
	StorageID        int64
	Name             *string
	Visibility       *domain.StorageVisibility
	MaxFileSize      *int64
	MaxStorageSize   *int64
	AllowedFileTypes *[]string
	BlockedFileTypes *[]string
}

type AdjustStorageSizeParams struct {
	StorageID int64
	Delta     int64
}
