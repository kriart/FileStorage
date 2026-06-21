package storage

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
	uow  *postgres.UnitOfWork
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
		uow:  postgres.NewUnitOfWork(pool),
	}
}

func (r *PostgresRepository) Create(ctx context.Context, params CreateStorageParams) (*domain.Storage, error) {
	var created *domain.Storage
	err := r.uow.WithinTx(ctx, func(ctx context.Context) error {
		const query = `
			INSERT INTO storages (name, type, visibility, max_file_size, max_storage_size)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, name, type, visibility, max_file_size, max_storage_size, used_size, created_at, updated_at, deleted_at
		`

		storage := new(domain.Storage)
		err := postgres.Executor(ctx, r.pool).QueryRow(
			ctx,
			query,
			params.Name,
			params.Type,
			params.Visibility,
			params.MaxFileSize,
			params.MaxStorageSize,
		).Scan(
			&storage.ID,
			&storage.Name,
			&storage.Type,
			&storage.Visibility,
			&storage.MaxFileSize,
			&storage.MaxStorageSize,
			&storage.UsedSize,
			&storage.CreatedAt,
			&storage.UpdatedAt,
			&storage.DeletedAt,
		)
		if err != nil {
			return err
		}

		if err := r.replaceTypeRules(ctx, storage.ID, buildTypeRules(storage.ID, params.AllowedFileTypes, params.BlockedFileTypes)); err != nil {
			return err
		}

		const ownerAccessQuery = `
			INSERT INTO storage_accesses (storage_id, user_id, access_level)
			VALUES ($1, $2, $3)
		`
		if _, err := postgres.Executor(ctx, r.pool).Exec(ctx, ownerAccessQuery, storage.ID, params.OwnerID, domain.StorageAccessOwner); err != nil {
			return err
		}

		created = storage
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int64) (*domain.Storage, error) {
	const query = `
		SELECT id, name, type, visibility, max_file_size, max_storage_size, used_size, created_at, updated_at, deleted_at
		FROM storages
		WHERE id = $1
	`

	return r.scanStorage(ctx, query, id)
}

func (r *PostgresRepository) ListAvailableForUser(ctx context.Context, filter ListStoragesFilter) ([]domain.Storage, error) {
	query := `
		SELECT DISTINCT s.id, s.name, s.type, s.visibility, s.max_file_size, s.max_storage_size, s.used_size, s.created_at, s.updated_at, s.deleted_at
		FROM storages s
		LEFT JOIN storage_accesses sa ON sa.storage_id = s.id AND sa.user_id = $1
		WHERE (sa.user_id IS NOT NULL OR s.visibility IN ('PUBLIC_READ', 'PUBLIC_UPLOAD'))
	`
	args := []any{filter.UserID}

	if !filter.IncludeDeleted {
		query += ` AND s.deleted_at IS NULL`
	}
	if filter.Type != nil {
		args = append(args, *filter.Type)
		query += ` AND s.type = $2`
	}
	query += ` ORDER BY s.id`

	rows, err := postgres.Executor(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	storages := make([]domain.Storage, 0)
	for rows.Next() {
		storage := domain.Storage{}
		if err := rows.Scan(
			&storage.ID,
			&storage.Name,
			&storage.Type,
			&storage.Visibility,
			&storage.MaxFileSize,
			&storage.MaxStorageSize,
			&storage.UsedSize,
			&storage.CreatedAt,
			&storage.UpdatedAt,
			&storage.DeletedAt,
		); err != nil {
			return nil, err
		}
		storages = append(storages, storage)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return storages, nil
}

func (r *PostgresRepository) Update(ctx context.Context, params UpdateStorageParams) (*domain.Storage, error) {
	var updated *domain.Storage
	err := r.uow.WithinTx(ctx, func(ctx context.Context) error {
		current, err := r.GetByID(ctx, params.StorageID)
		if err != nil {
			return err
		}

		name := current.Name
		visibility := current.Visibility
		maxFileSize := current.MaxFileSize
		maxStorageSize := current.MaxStorageSize

		if params.Name != nil {
			name = *params.Name
		}
		if params.Visibility != nil {
			visibility = *params.Visibility
		}
		if params.MaxFileSize != nil {
			maxFileSize = *params.MaxFileSize
		}
		if params.MaxStorageSize != nil {
			maxStorageSize = *params.MaxStorageSize
		}
		if maxStorageSize < current.UsedSize {
			return repository.ErrLimitExceeded
		}

		const query = `
			UPDATE storages
			SET name = $2, visibility = $3, max_file_size = $4, max_storage_size = $5, updated_at = now()
			WHERE id = $1 AND deleted_at IS NULL
			RETURNING id, name, type, visibility, max_file_size, max_storage_size, used_size, created_at, updated_at, deleted_at
		`

		storage := new(domain.Storage)
		err = postgres.Executor(ctx, r.pool).QueryRow(
			ctx,
			query,
			params.StorageID,
			name,
			visibility,
			maxFileSize,
			maxStorageSize,
		).Scan(
			&storage.ID,
			&storage.Name,
			&storage.Type,
			&storage.Visibility,
			&storage.MaxFileSize,
			&storage.MaxStorageSize,
			&storage.UsedSize,
			&storage.CreatedAt,
			&storage.UpdatedAt,
			&storage.DeletedAt,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return repository.ErrNotFound
			}
			return err
		}

		if params.AllowedFileTypes != nil || params.BlockedFileTypes != nil {
			rules, err := r.ListTypeRules(ctx, params.StorageID)
			if err != nil {
				return err
			}
			allowedTypes, blockedTypes := splitTypeRules(rules)
			if params.AllowedFileTypes != nil {
				allowedTypes = *params.AllowedFileTypes
			}
			if params.BlockedFileTypes != nil {
				blockedTypes = *params.BlockedFileTypes
			}
			if err := r.replaceTypeRules(ctx, params.StorageID, buildTypeRules(params.StorageID, allowedTypes, blockedTypes)); err != nil {
				return err
			}
		}

		updated = storage
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *PostgresRepository) SoftDelete(ctx context.Context, id int64) error {
	const query = `UPDATE storages SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`

	tag, err := postgres.Executor(ctx, r.pool).Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) ListTypeRules(ctx context.Context, storageID int64) ([]domain.StorageTypeRule, error) {
	const query = `
		SELECT id, storage_id, rule_type, pattern, created_at
		FROM storage_type_rules
		WHERE storage_id = $1
		ORDER BY rule_type, pattern
	`

	rows, err := postgres.Executor(ctx, r.pool).Query(ctx, query, storageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]domain.StorageTypeRule, 0)
	for rows.Next() {
		rule := domain.StorageTypeRule{}
		if err := rows.Scan(&rule.ID, &rule.StorageID, &rule.RuleType, &rule.Pattern, &rule.CreatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *PostgresRepository) ReplaceTypeRules(ctx context.Context, storageID int64, rules []domain.StorageTypeRule) error {
	return r.uow.WithinTx(ctx, func(ctx context.Context) error {
		return r.replaceTypeRules(ctx, storageID, rules)
	})
}

func (r *PostgresRepository) AdjustUsedSize(ctx context.Context, params AdjustStorageSizeParams) (*domain.Storage, error) {
	const query = `
		UPDATE storages
		SET used_size = used_size + $2, updated_at = now()
		WHERE id = $1
			AND deleted_at IS NULL
			AND used_size + $2 >= 0
			AND used_size + $2 <= max_storage_size
		RETURNING id, name, type, visibility, max_file_size, max_storage_size, used_size, created_at, updated_at, deleted_at
	`

	storage := new(domain.Storage)
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, params.StorageID, params.Delta).Scan(
		&storage.ID,
		&storage.Name,
		&storage.Type,
		&storage.Visibility,
		&storage.MaxFileSize,
		&storage.MaxStorageSize,
		&storage.UsedSize,
		&storage.CreatedAt,
		&storage.UpdatedAt,
		&storage.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrLimitExceeded
		}
		return nil, err
	}
	return storage, nil
}

func (r *PostgresRepository) replaceTypeRules(ctx context.Context, storageID int64, rules []domain.StorageTypeRule) error {
	if _, err := postgres.Executor(ctx, r.pool).Exec(ctx, `DELETE FROM storage_type_rules WHERE storage_id = $1`, storageID); err != nil {
		return err
	}

	for _, rule := range rules {
		if rule.Pattern == "" {
			continue
		}
		const query = `
			INSERT INTO storage_type_rules (storage_id, rule_type, pattern)
			VALUES ($1, $2, $3)
			ON CONFLICT (storage_id, rule_type, pattern) DO NOTHING
		`
		if _, err := postgres.Executor(ctx, r.pool).Exec(ctx, query, storageID, rule.RuleType, rule.Pattern); err != nil {
			return err
		}
	}
	return nil
}

func buildTypeRules(storageID int64, allowedTypes []string, blockedTypes []string) []domain.StorageTypeRule {
	rules := make([]domain.StorageTypeRule, 0, len(allowedTypes)+len(blockedTypes))
	for _, pattern := range allowedTypes {
		rules = append(rules, domain.StorageTypeRule{StorageID: storageID, RuleType: domain.StorageTypeRuleAllow, Pattern: pattern})
	}
	for _, pattern := range blockedTypes {
		rules = append(rules, domain.StorageTypeRule{StorageID: storageID, RuleType: domain.StorageTypeRuleDeny, Pattern: pattern})
	}
	return rules
}

func splitTypeRules(rules []domain.StorageTypeRule) ([]string, []string) {
	allowedTypes := make([]string, 0)
	blockedTypes := make([]string, 0)
	for _, rule := range rules {
		switch rule.RuleType {
		case domain.StorageTypeRuleAllow:
			allowedTypes = append(allowedTypes, rule.Pattern)
		case domain.StorageTypeRuleDeny:
			blockedTypes = append(blockedTypes, rule.Pattern)
		}
	}
	return allowedTypes, blockedTypes
}

func (r *PostgresRepository) scanStorage(ctx context.Context, query string, args ...any) (*domain.Storage, error) {
	storage := new(domain.Storage)
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, args...).Scan(
		&storage.ID,
		&storage.Name,
		&storage.Type,
		&storage.Visibility,
		&storage.MaxFileSize,
		&storage.MaxStorageSize,
		&storage.UsedSize,
		&storage.CreatedAt,
		&storage.UpdatedAt,
		&storage.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return storage, nil
}
