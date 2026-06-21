package domain

import "time"

type UserRole string

const (
	UserRoleAdmin UserRole = "ADMIN"
	UserRoleUser  UserRole = "USER"
)

type StorageType string

const (
	StorageTypePersonal StorageType = "PERSONAL"
	StorageTypeGlobal   StorageType = "GLOBAL"
)

type StorageVisibility string

const (
	StorageVisibilityPrivate      StorageVisibility = "PRIVATE"
	StorageVisibilityPublicRead   StorageVisibility = "PUBLIC_READ"
	StorageVisibilityPublicUpload StorageVisibility = "PUBLIC_UPLOAD"
)

type StorageAccessLevel string

const (
	StorageAccessOwner    StorageAccessLevel = "OWNER"
	StorageAccessManager  StorageAccessLevel = "MANAGER"
	StorageAccessUploader StorageAccessLevel = "UPLOADER"
	StorageAccessViewer   StorageAccessLevel = "VIEWER"
)

type ShareAccessType string

const (
	ShareAccessRead  ShareAccessType = "READ"
	ShareAccessWrite ShareAccessType = "WRITE"
)

type User struct {
	ID           int64
	Username     string
	Email        string
	PasswordHash string
	Role         UserRole
	AvatarPath   *string
	DateOfBirth  *time.Time
	Theme        string
	Language     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Storage struct {
	ID             int64
	Name           string
	Type           StorageType
	Visibility     StorageVisibility
	MaxFileSize    int64
	MaxStorageSize int64
	UsedSize       int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

type StorageTypeRuleType string

const (
	StorageTypeRuleAllow StorageTypeRuleType = "ALLOW"
	StorageTypeRuleDeny  StorageTypeRuleType = "DENY"
)

type StorageTypeRule struct {
	ID        int64
	StorageID int64
	RuleType  StorageTypeRuleType
	Pattern   string
	CreatedAt time.Time
}

type StorageAccess struct {
	ID          int64
	StorageID   int64
	UserID      int64
	AccessLevel StorageAccessLevel
	CreatedAt   time.Time
}

type FolderAccess struct {
	ID          int64
	FolderID    int64
	UserID      int64
	AccessLevel StorageAccessLevel
	CreatedAt   time.Time
}

type File struct {
	ID           int64
	StorageID    int64
	FolderID     *int64
	OwnerID      int64
	OriginalName string
	StoredName   string
	RelativePath string
	MimeType     string
	Size         int64
	Checksum     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

type Folder struct {
	ID        int64
	StorageID int64
	ParentID  *int64
	Name      string
	CreatedBy int64
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type ShareLink struct {
	ID         int64
	FileID     int64
	Token      *string
	TokenHash  string
	AccessType ShareAccessType
	ExpiresAt  *time.Time
	UseCount   int
	CreatedBy  int64
	IsActive   bool
	CreatedAt  time.Time
}

type RefreshToken struct {
	ID        int64
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
	RotatedAt *time.Time
}
