package access

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

func (r *PostgresRepository) UpsertStorageAccess(ctx context.Context, params UpsertStorageAccessParams) (*domain.StorageAccess, error) {
	const query = `
		INSERT INTO storage_accesses (storage_id, user_id, access_level)
		VALUES ($1, $2, $3)
		ON CONFLICT (storage_id, user_id)
		DO UPDATE SET access_level = EXCLUDED.access_level
		RETURNING id, storage_id, user_id, access_level, created_at
	`

	access := new(domain.StorageAccess)
	err := postgres.Executor(ctx, r.pool).QueryRow(
		ctx,
		query,
		params.StorageID,
		params.UserID,
		params.AccessLevel,
	).Scan(&access.ID, &access.StorageID, &access.UserID, &access.AccessLevel, &access.CreatedAt)
	if err != nil {
		return nil, err
	}
	return access, nil
}

func (r *PostgresRepository) GetStorageAccess(ctx context.Context, storageID int64, userID int64) (*domain.StorageAccess, error) {
	const query = `
		SELECT id, storage_id, user_id, access_level, created_at
		FROM storage_accesses
		WHERE storage_id = $1 AND user_id = $2
	`

	access := new(domain.StorageAccess)
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, storageID, userID).Scan(
		&access.ID,
		&access.StorageID,
		&access.UserID,
		&access.AccessLevel,
		&access.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return access, nil
}

func (r *PostgresRepository) ListStorageAccesses(ctx context.Context, storageID int64) ([]domain.StorageAccess, error) {
	const query = `
		SELECT id, storage_id, user_id, access_level, created_at
		FROM storage_accesses
		WHERE storage_id = $1
		ORDER BY access_level, user_id
	`

	rows, err := postgres.Executor(ctx, r.pool).Query(ctx, query, storageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accesses := make([]domain.StorageAccess, 0)
	for rows.Next() {
		access := domain.StorageAccess{}
		if err := rows.Scan(&access.ID, &access.StorageID, &access.UserID, &access.AccessLevel, &access.CreatedAt); err != nil {
			return nil, err
		}
		accesses = append(accesses, access)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accesses, nil
}

func (r *PostgresRepository) DeleteStorageAccess(ctx context.Context, storageID int64, userID int64) error {
	const query = `DELETE FROM storage_accesses WHERE storage_id = $1 AND user_id = $2`

	tag, err := postgres.Executor(ctx, r.pool).Exec(ctx, query, storageID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) UpsertFolderAccess(ctx context.Context, params UpsertFolderAccessParams) (*domain.FolderAccess, error) {
	const query = `
		INSERT INTO folder_accesses (folder_id, user_id, access_level)
		VALUES ($1, $2, $3)
		ON CONFLICT (folder_id, user_id)
		DO UPDATE SET access_level = EXCLUDED.access_level
		RETURNING id, folder_id, user_id, access_level, created_at
	`

	access := new(domain.FolderAccess)
	err := postgres.Executor(ctx, r.pool).QueryRow(
		ctx,
		query,
		params.FolderID,
		params.UserID,
		params.AccessLevel,
	).Scan(&access.ID, &access.FolderID, &access.UserID, &access.AccessLevel, &access.CreatedAt)
	if err != nil {
		return nil, err
	}
	return access, nil
}

func (r *PostgresRepository) GetFolderAccess(ctx context.Context, folderID int64, userID int64) (*domain.FolderAccess, error) {
	const query = `
		SELECT id, folder_id, user_id, access_level, created_at
		FROM folder_accesses
		WHERE folder_id = $1 AND user_id = $2
	`

	access := new(domain.FolderAccess)
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, folderID, userID).Scan(
		&access.ID,
		&access.FolderID,
		&access.UserID,
		&access.AccessLevel,
		&access.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return access, nil
}

func (r *PostgresRepository) ListFolderAccesses(ctx context.Context, folderID int64) ([]domain.FolderAccess, error) {
	const query = `
		SELECT id, folder_id, user_id, access_level, created_at
		FROM folder_accesses
		WHERE folder_id = $1
		ORDER BY access_level, user_id
	`

	rows, err := postgres.Executor(ctx, r.pool).Query(ctx, query, folderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accesses := make([]domain.FolderAccess, 0)
	for rows.Next() {
		access := domain.FolderAccess{}
		if err := rows.Scan(&access.ID, &access.FolderID, &access.UserID, &access.AccessLevel, &access.CreatedAt); err != nil {
			return nil, err
		}
		accesses = append(accesses, access)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accesses, nil
}

func (r *PostgresRepository) DeleteFolderAccess(ctx context.Context, folderID int64, userID int64) error {
	const query = `DELETE FROM folder_accesses WHERE folder_id = $1 AND user_id = $2`

	tag, err := postgres.Executor(ctx, r.pool).Exec(ctx, query, folderID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}
