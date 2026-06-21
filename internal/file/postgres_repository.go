package file

import (
	"context"
	"errors"
	"strconv"
	"strings"

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

func (r *PostgresRepository) Create(ctx context.Context, params CreateFileParams) (*domain.File, error) {
	const query = `
		INSERT INTO files (storage_id, folder_id, owner_id, original_name, stored_name, relative_path, mime_type, size, checksum)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, storage_id, folder_id, owner_id, original_name, stored_name, relative_path, mime_type, size, checksum, created_at, updated_at, deleted_at
	`

	return r.scanFile(ctx, query,
		params.StorageID,
		params.FolderID,
		params.OwnerID,
		params.OriginalName,
		params.StoredName,
		params.RelativePath,
		params.MimeType,
		params.Size,
		params.Checksum,
	)
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int64) (*domain.File, error) {
	const query = `
		SELECT id, storage_id, folder_id, owner_id, original_name, stored_name, relative_path, mime_type, size, checksum, created_at, updated_at, deleted_at
		FROM files
		WHERE id = $1
	`

	return r.scanFile(ctx, query, id)
}

func (r *PostgresRepository) ListByStorage(ctx context.Context, filter ListFilesFilter) ([]domain.File, error) {
	query := `
		SELECT id, storage_id, folder_id, owner_id, original_name, stored_name, relative_path, mime_type, size, checksum, created_at, updated_at, deleted_at
		FROM files
		WHERE storage_id = $1
	`
	args := []any{filter.StorageID}
	if filter.FolderID == nil {
		query += ` AND folder_id IS NULL`
	} else {
		args = append(args, *filter.FolderID)
		query += ` AND folder_id = $` + argPos(len(args))
	}
	if !filter.IncludeDeleted {
		query += ` AND deleted_at IS NULL`
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		query += ` AND lower(original_name) LIKE $` + argPos(len(args))
	}
	query += ` ORDER BY ` + fileOrderBy(filter.Sort, filter.Direction)
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += ` LIMIT $` + argPos(len(args))
	}
	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		query += ` OFFSET $` + argPos(len(args))
	}

	rows, err := postgres.Executor(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]domain.File, 0)
	for rows.Next() {
		file := domain.File{}
		if err := rows.Scan(
			&file.ID,
			&file.StorageID,
			&file.FolderID,
			&file.OwnerID,
			&file.OriginalName,
			&file.StoredName,
			&file.RelativePath,
			&file.MimeType,
			&file.Size,
			&file.Checksum,
			&file.CreatedAt,
			&file.UpdatedAt,
			&file.DeletedAt,
		); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return files, nil
}

func (r *PostgresRepository) CountByStorage(ctx context.Context, filter ListFilesFilter) (int, error) {
	query := `SELECT count(*) FROM files WHERE storage_id = $1`
	args := []any{filter.StorageID}
	if filter.FolderID == nil {
		query += ` AND folder_id IS NULL`
	} else {
		args = append(args, *filter.FolderID)
		query += ` AND folder_id = $` + argPos(len(args))
	}
	if !filter.IncludeDeleted {
		query += ` AND deleted_at IS NULL`
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		query += ` AND lower(original_name) LIKE $` + argPos(len(args))
	}

	var total int
	if err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *PostgresRepository) UpdateContent(ctx context.Context, params UpdateFileContentParams) (*domain.File, error) {
	const query = `
		UPDATE files
		SET original_name = $2,
			stored_name = $3,
			relative_path = $4,
			mime_type = $5,
			size = $6,
			checksum = $7,
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, storage_id, folder_id, owner_id, original_name, stored_name, relative_path, mime_type, size, checksum, created_at, updated_at, deleted_at
	`

	return r.scanFile(ctx, query,
		params.FileID,
		params.OriginalName,
		params.StoredName,
		params.RelativePath,
		params.MimeType,
		params.Size,
		params.Checksum,
	)
}

func (r *PostgresRepository) Rename(ctx context.Context, params RenameFileParams) (*domain.File, error) {
	const query = `
		UPDATE files
		SET original_name = $2,
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, storage_id, folder_id, owner_id, original_name, stored_name, relative_path, mime_type, size, checksum, created_at, updated_at, deleted_at
	`

	return r.scanFile(ctx, query, params.FileID, params.OriginalName)
}

func argPos(pos int) string {
	return strconv.Itoa(pos)
}

func fileOrderBy(sort string, direction string) string {
	dir := "ASC"
	if strings.EqualFold(direction, "desc") {
		dir = "DESC"
	}
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "name":
		return "lower(original_name) " + dir + ", id DESC"
	case "type":
		return "mime_type " + dir + ", lower(original_name) ASC"
	case "size":
		return "size " + dir + ", lower(original_name) ASC"
	case "created":
		return "created_at " + dir + ", id DESC"
	default:
		return "id DESC"
	}
}

func (r *PostgresRepository) SoftDelete(ctx context.Context, params SoftDeleteFileParams) (*domain.File, error) {
	const query = `
		UPDATE files
		SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, storage_id, folder_id, owner_id, original_name, stored_name, relative_path, mime_type, size, checksum, created_at, updated_at, deleted_at
	`

	return r.scanFile(ctx, query, params.FileID)
}

func (r *PostgresRepository) scanFile(ctx context.Context, query string, args ...any) (*domain.File, error) {
	file := new(domain.File)
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, args...).Scan(
		&file.ID,
		&file.StorageID,
		&file.FolderID,
		&file.OwnerID,
		&file.OriginalName,
		&file.StoredName,
		&file.RelativePath,
		&file.MimeType,
		&file.Size,
		&file.Checksum,
		&file.CreatedAt,
		&file.UpdatedAt,
		&file.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return file, nil
}
