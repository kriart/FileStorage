package folder

import "file-storage-server/internal/domain"

type CreateFolderDTO struct {
	StorageID int64  `json:"storageId"`
	ParentID  *int64 `json:"parentId,omitempty"`
	Name      string `json:"name"`
}

type FolderDTO struct {
	ID        int64  `json:"id"`
	StorageID int64  `json:"storageId"`
	ParentID  *int64 `json:"parentId,omitempty"`
	Name      string `json:"name"`
}

type RenameFolderDTO struct {
	Name string `json:"name"`
}

type ListFoldersFilter struct {
	StorageID      int64
	ParentID       *int64
	IncludeDeleted bool
}

type CreateFolderParams struct {
	StorageID int64
	ParentID  *int64
	Name      string
	CreatedBy int64
}

type RenameFolderParams struct {
	FolderID int64
	Name     string
}

type DeleteFolderTreeResult struct {
	Folder          domain.Folder
	DeletedFileSize int64
}
