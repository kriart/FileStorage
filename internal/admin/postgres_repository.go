package admin

import (
	"context"

	"file-storage-server/internal/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Dashboard(ctx context.Context) (*Dashboard, error) {
	stats, err := r.stats(ctx)
	if err != nil {
		return nil, err
	}
	users, err := r.users(ctx)
	if err != nil {
		return nil, err
	}
	storages, err := r.storages(ctx)
	if err != nil {
		return nil, err
	}
	auditRows, err := r.audit(ctx)
	if err != nil {
		return nil, err
	}
	return &Dashboard{Stats: *stats, Users: users, Storages: storages, Audit: auditRows}, nil
}

func (r *Repository) stats(ctx context.Context) (*Stats, error) {
	const query = `
		SELECT
			(SELECT count(*) FROM users)::int,
			(SELECT count(*) FROM storages WHERE deleted_at IS NULL)::int,
			(SELECT count(*) FROM files WHERE deleted_at IS NULL)::int,
			(SELECT count(*) FROM share_links WHERE is_active = true)::int,
			(SELECT COALESCE(sum(used_size), 0) FROM storages WHERE deleted_at IS NULL)::bigint
	`
	stats := new(Stats)
	if err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query).Scan(
		&stats.Users,
		&stats.Storages,
		&stats.Files,
		&stats.ActiveLinks,
		&stats.UsedBytes,
	); err != nil {
		return nil, err
	}
	return stats, nil
}

func (r *Repository) users(ctx context.Context) ([]UserRow, error) {
	const query = `
		SELECT id, email, username, role, created_at
		FROM users
		ORDER BY id DESC
		LIMIT 20
	`
	rows, err := postgres.Executor(ctx, r.pool).Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]UserRow, 0)
	for rows.Next() {
		row := UserRow{}
		if err := rows.Scan(&row.ID, &row.Email, &row.Username, &row.Role, &row.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *Repository) storages(ctx context.Context) ([]StorageRow, error) {
	const query = `
		SELECT id, name, type, used_size, deleted_at IS NOT NULL, created_at
		FROM storages
		ORDER BY id DESC
		LIMIT 20
	`
	rows, err := postgres.Executor(ctx, r.pool).Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]StorageRow, 0)
	for rows.Next() {
		row := StorageRow{}
		if err := rows.Scan(&row.ID, &row.Name, &row.Type, &row.UsedSize, &row.Deleted, &row.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *Repository) audit(ctx context.Context) ([]AuditRow, error) {
	const query = `
		SELECT a.id, u.email, a.action, a.entity_type, a.entity_id, COALESCE(a.ip, ''), a.created_at
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.user_id
		ORDER BY a.id DESC
		LIMIT 50
	`
	rows, err := postgres.Executor(ctx, r.pool).Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]AuditRow, 0)
	for rows.Next() {
		row := AuditRow{}
		if err := rows.Scan(&row.ID, &row.UserEmail, &row.Action, &row.EntityType, &row.EntityID, &row.IP, &row.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
