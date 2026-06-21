package folder

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

func (r *PostgresRepository) Create(ctx context.Context, params CreateFolderParams) (*domain.Folder, error) {
	const query = `
		INSERT INTO folders (storage_id, parent_id, name, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, storage_id, parent_id, name, created_by, created_at, updated_at, deleted_at
	`

	return r.scanFolder(ctx, query, params.StorageID, params.ParentID, params.Name, params.CreatedBy)
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int64) (*domain.Folder, error) {
	const query = `
		SELECT id, storage_id, parent_id, name, created_by, created_at, updated_at, deleted_at
		FROM folders
		WHERE id = $1
	`

	return r.scanFolder(ctx, query, id)
}

func (r *PostgresRepository) ListByStorage(ctx context.Context, filter ListFoldersFilter) ([]domain.Folder, error) {
	query := `
		SELECT id, storage_id, parent_id, name, created_by, created_at, updated_at, deleted_at
		FROM folders
		WHERE storage_id = $1
	`
	args := []any{filter.StorageID}
	if filter.ParentID == nil {
		query += ` AND parent_id IS NULL`
	} else {
		args = append(args, *filter.ParentID)
		query += ` AND parent_id = $2`
	}
	if !filter.IncludeDeleted {
		query += ` AND deleted_at IS NULL`
	}
	query += ` ORDER BY lower(name), id`

	rows, err := postgres.Executor(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	folders := make([]domain.Folder, 0)
	for rows.Next() {
		folder := domain.Folder{}
		if err := rows.Scan(
			&folder.ID,
			&folder.StorageID,
			&folder.ParentID,
			&folder.Name,
			&folder.CreatedBy,
			&folder.CreatedAt,
			&folder.UpdatedAt,
			&folder.DeletedAt,
		); err != nil {
			return nil, err
		}
		folders = append(folders, folder)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return folders, nil
}

func (r *PostgresRepository) Rename(ctx context.Context, params RenameFolderParams) (*domain.Folder, error) {
	const query = `
		UPDATE folders
		SET name = $2, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, storage_id, parent_id, name, created_by, created_at, updated_at, deleted_at
	`

	return r.scanFolder(ctx, query, params.FolderID, params.Name)
}

func (r *PostgresRepository) SoftDeleteTree(ctx context.Context, id int64) (*DeleteFolderTreeResult, error) {
	folder, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if folder.DeletedAt != nil {
		return nil, repository.ErrNotFound
	}

	const query = `
		WITH RECURSIVE tree AS (
			SELECT id
			FROM folders
			WHERE id = $1 AND deleted_at IS NULL
			UNION ALL
			SELECT child.id
			FROM folders child
			JOIN tree ON child.parent_id = tree.id
			WHERE child.deleted_at IS NULL
		),
		deleted_files AS (
			UPDATE files
			SET deleted_at = now(), updated_at = now()
			WHERE folder_id IN (SELECT id FROM tree)
				AND deleted_at IS NULL
			RETURNING size
		),
		deleted_folders AS (
			UPDATE folders
			SET deleted_at = now(), updated_at = now()
			WHERE id IN (SELECT id FROM tree)
			RETURNING id
		)
		SELECT COALESCE(SUM(size), 0)::bigint FROM deleted_files
	`

	var deletedFileSize int64
	if err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, id).Scan(&deletedFileSize); err != nil {
		return nil, err
	}

	folder.DeletedAt = nil
	return &DeleteFolderTreeResult{Folder: *folder, DeletedFileSize: deletedFileSize}, nil
}

func (r *PostgresRepository) scanFolder(ctx context.Context, query string, args ...any) (*domain.Folder, error) {
	folder := new(domain.Folder)
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, args...).Scan(
		&folder.ID,
		&folder.StorageID,
		&folder.ParentID,
		&folder.Name,
		&folder.CreatedBy,
		&folder.CreatedAt,
		&folder.UpdatedAt,
		&folder.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return folder, nil
}
