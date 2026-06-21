package share

import (
	"time"

	"file-storage-server/internal/domain"
)

type CreateShareLinkDTO struct {
	AccessType domain.ShareAccessType `json:"accessType"`
	ExpiresAt  *time.Time             `json:"expiresAt,omitempty"`
}

type ShareLinkDTO struct {
	ID         int64                  `json:"id"`
	FileID     int64                  `json:"fileId"`
	URL        string                 `json:"url,omitempty"`
	AccessType domain.ShareAccessType `json:"accessType"`
	ExpiresAt  *time.Time             `json:"expiresAt,omitempty"`
	UseCount   int                    `json:"useCount"`
	IsActive   bool                   `json:"isActive"`
}

type PublicShareDTO struct {
	FileID       int64                  `json:"fileId"`
	OriginalName string                 `json:"originalName"`
	MimeType     string                 `json:"mimeType"`
	Size         int64                  `json:"size"`
	AccessType   domain.ShareAccessType `json:"accessType"`
}

type CreateShareLinkParams struct {
	FileID     int64
	Token      string
	TokenHash  string
	AccessType domain.ShareAccessType
	ExpiresAt  *time.Time
	CreatedBy  int64
}

type ListShareLinksFilter struct {
	FileID          int64
	IncludeInactive bool
}
