package share

import (
	"context"

	"file-storage-server/internal/domain"
)

type Repository interface {
	Create(ctx context.Context, params CreateShareLinkParams) (*domain.ShareLink, error)
	GetByID(ctx context.Context, id int64) (*domain.ShareLink, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*domain.ShareLink, error)
	ListByFile(ctx context.Context, filter ListShareLinksFilter) ([]domain.ShareLink, error)
	UpdateToken(ctx context.Context, id int64, token string, tokenHash string) (*domain.ShareLink, error)
	IncrementUseCount(ctx context.Context, id int64) error
	Deactivate(ctx context.Context, id int64) error
}
