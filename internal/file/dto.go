package file

type UploadFileDTO struct {
	StorageID    int64
	FolderID     *int64
	OwnerID      int64
	OriginalName string
	MimeType     string
	Size         int64
}

type ReplaceFileDTO struct {
	FileID       int64
	ActorID      int64
	OriginalName string
	MimeType     string
	Size         int64
}

type RenameFileDTO struct {
	Name string `json:"name"`
}

type FileDTO struct {
	ID           int64  `json:"id"`
	StorageID    int64  `json:"storageId"`
	FolderID     *int64 `json:"folderId,omitempty"`
	OwnerID      int64  `json:"ownerId"`
	OriginalName string `json:"originalName"`
	MimeType     string `json:"mimeType"`
	Size         int64  `json:"size"`
	Checksum     string `json:"checksum"`
}

type ListFilesFilter struct {
	StorageID      int64
	FolderID       *int64
	IncludeDeleted bool
	Search         string
	Sort           string
	Direction      string
	Limit          int
	Offset         int
}

type ListFilesPage struct {
	Files  []FileDTO
	Total  int
	Limit  int
	Offset int
}

type CreateFileParams struct {
	StorageID    int64
	FolderID     *int64
	OwnerID      int64
	OriginalName string
	StoredName   string
	RelativePath string
	MimeType     string
	Size         int64
	Checksum     string
}

type UpdateFileContentParams struct {
	FileID       int64
	OriginalName string
	StoredName   string
	RelativePath string
	MimeType     string
	Size         int64
	Checksum     string
}

type RenameFileParams struct {
	FileID       int64
	OriginalName string
}

type SoftDeleteFileParams struct {
	FileID  int64
	ActorID int64
}
