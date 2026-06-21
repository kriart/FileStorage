package share

import (
	"context"
	"errors"

	"file-storage-server/internal/domain"
	"file-storage-server/internal/postgres"
	"file-storage-server/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, params CreateShareLinkParams) (*domain.ShareLink, error) {
	const query = `
		INSERT INTO share_links (file_id, token, token_hash, access_type, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, file_id, token, token_hash, access_type, expires_at, use_count, created_by, is_active, created_at
	`

	return r.scanShareLink(ctx, query,
		params.FileID,
		params.Token,
		params.TokenHash,
		params.AccessType,
		params.ExpiresAt,
		params.CreatedBy,
	)
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int64) (*domain.ShareLink, error) {
	const query = `
		SELECT id, file_id, token, token_hash, access_type, expires_at, use_count, created_by, is_active, created_at
		FROM share_links
		WHERE id = $1
	`

	return r.scanShareLink(ctx, query, id)
}

func (r *PostgresRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.ShareLink, error) {
	const query = `
		SELECT id, file_id, token, token_hash, access_type, expires_at, use_count, created_by, is_active, created_at
		FROM share_links
		WHERE token_hash = $1
			AND is_active = true
			AND (expires_at IS NULL OR expires_at > now())
	`

	return r.scanShareLink(ctx, query, tokenHash)
}

func (r *PostgresRepository) ListByFile(ctx context.Context, filter ListShareLinksFilter) ([]domain.ShareLink, error) {
	query := `
		SELECT id, file_id, token, token_hash, access_type, expires_at, use_count, created_by, is_active, created_at
		FROM share_links
		WHERE file_id = $1
	`
	if !filter.IncludeInactive {
		query += ` AND is_active = true`
	}
	query += ` ORDER BY id DESC`

	rows, err := postgres.Executor(ctx, r.pool).Query(ctx, query, filter.FileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	links := make([]domain.ShareLink, 0)
	for rows.Next() {
		link := domain.ShareLink{}
		if err := rows.Scan(
			&link.ID,
			&link.FileID,
			&link.Token,
			&link.TokenHash,
			&link.AccessType,
			&link.ExpiresAt,
			&link.UseCount,
			&link.CreatedBy,
			&link.IsActive,
			&link.CreatedAt,
		); err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return links, nil
}

func (r *PostgresRepository) UpdateToken(ctx context.Context, id int64, token string, tokenHash string) (*domain.ShareLink, error) {
	const query = `
		UPDATE share_links
		SET token = $2, token_hash = $3
		WHERE id = $1
			AND is_active = true
		RETURNING id, file_id, token, token_hash, access_type, expires_at, use_count, created_by, is_active, created_at
	`

	return r.scanShareLink(ctx, query, id, token, tokenHash)
}

func (r *PostgresRepository) IncrementUseCount(ctx context.Context, id int64) error {
	const query = `
		UPDATE share_links
		SET use_count = use_count + 1
		WHERE id = $1
			AND is_active = true
			AND (expires_at IS NULL OR expires_at > now())
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

func (r *PostgresRepository) Deactivate(ctx context.Context, id int64) error {
	const query = `UPDATE share_links SET is_active = false WHERE id = $1 AND is_active = true`

	tag, err := postgres.Executor(ctx, r.pool).Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) scanShareLink(ctx context.Context, query string, args ...any) (*domain.ShareLink, error) {
	link := new(domain.ShareLink)
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, args...).Scan(
		&link.ID,
		&link.FileID,
		&link.Token,
		&link.TokenHash,
		&link.AccessType,
		&link.ExpiresAt,
		&link.UseCount,
		&link.CreatedBy,
		&link.IsActive,
		&link.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return link, nil
}
