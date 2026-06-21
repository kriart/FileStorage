package auth

import (
	"context"
	"time"

	"file-storage-server/internal/domain"
)

type RefreshTokenRepository interface {
	CreateRefreshToken(ctx context.Context, params CreateRefreshTokenParams) (*domain.RefreshToken, error)
	GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)
	RotateRefreshToken(ctx context.Context, tokenHash string, newTokenHash string, newExpiresAt time.Time, now time.Time) (*domain.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id int64) error
}

type CreateRefreshTokenParams struct {
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
}
