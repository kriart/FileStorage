package access

import "file-storage-server/internal/domain"

type GrantStorageAccessDTO struct {
	UserID      int64                     `json:"userId"`
	UserEmail   string                    `json:"userEmail,omitempty"`
	AccessLevel domain.StorageAccessLevel `json:"accessLevel"`
}

type UpdateStorageAccessDTO struct {
	AccessLevel domain.StorageAccessLevel `json:"accessLevel"`
}

type StorageAccessDTO struct {
	UserID      int64                     `json:"userId"`
	UserEmail   string                    `json:"userEmail"`
	StorageID   int64                     `json:"storageId"`
	AccessLevel domain.StorageAccessLevel `json:"accessLevel"`
}

type FolderAccessDTO struct {
	UserID      int64                     `json:"userId"`
	UserEmail   string                    `json:"userEmail"`
	FolderID    int64                     `json:"folderId"`
	AccessLevel domain.StorageAccessLevel `json:"accessLevel"`
}

type UpsertStorageAccessParams struct {
	StorageID   int64
	UserID      int64
	AccessLevel domain.StorageAccessLevel
}

type UpsertFolderAccessParams struct {
	FolderID    int64
	UserID      int64
	AccessLevel domain.StorageAccessLevel
}
