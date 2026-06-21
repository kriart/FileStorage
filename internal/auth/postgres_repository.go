package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"file-storage-server/internal/domain"
	"file-storage-server/internal/postgres"
	"file-storage-server/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRefreshTokenRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRefreshTokenRepository(pool *pgxpool.Pool) *PostgresRefreshTokenRepository {
	return &PostgresRefreshTokenRepository{pool: pool}
}

func (r *PostgresRefreshTokenRepository) CreateRefreshToken(ctx context.Context, params CreateRefreshTokenParams) (*domain.RefreshToken, error) {
	const query = `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token_hash, expires_at, revoked_at, created_at, rotated_at
	`
	return r.scanRefreshToken(ctx, query, params.UserID, params.TokenHash, params.ExpiresAt)
}

func (r *PostgresRefreshTokenRepository) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	const query = `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at, rotated_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	return r.scanRefreshToken(ctx, query, tokenHash)
}

func (r *PostgresRefreshTokenRepository) RotateRefreshToken(ctx context.Context, tokenHash string, newTokenHash string, newExpiresAt time.Time, now time.Time) (*domain.RefreshToken, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin refresh rotation: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const selectQuery = `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at, rotated_at
		FROM refresh_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`
	stored := new(domain.RefreshToken)
	err = tx.QueryRow(ctx, selectQuery, tokenHash).Scan(
		&stored.ID,
		&stored.UserID,
		&stored.TokenHash,
		&stored.ExpiresAt,
		&stored.RevokedAt,
		&stored.CreatedAt,
		&stored.RotatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	if stored.RevokedAt != nil || !stored.ExpiresAt.After(now) {
		return nil, repository.ErrUnauthorized
	}

	tag, err := tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = $2, rotated_at = $2
		WHERE id = $1 AND revoked_at IS NULL
	`, stored.ID, now)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, repository.ErrUnauthorized
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, stored.UserID, newTokenHash, newExpiresAt); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit refresh rotation: %w", err)
	}
	return stored, nil
}

func (r *PostgresRefreshTokenRepository) RevokeRefreshToken(ctx context.Context, id int64) error {
	const query = `
		UPDATE refresh_tokens
		SET revoked_at = COALESCE(revoked_at, now()), rotated_at = now()
		WHERE id = $1
	`
	tag, err := postgres.Executor(ctx, r.pool).Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *PostgresRefreshTokenRepository) scanRefreshToken(ctx context.Context, query string, args ...any) (*domain.RefreshToken, error) {
	token := new(domain.RefreshToken)
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, args...).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.RevokedAt,
		&token.CreatedAt,
		&token.RotatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return token, nil
}
