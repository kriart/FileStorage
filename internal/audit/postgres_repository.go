package audit

import (
	"context"

	"file-storage-server/internal/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Log(ctx context.Context, event Event) error {
	metadata, err := metadataJSON(event.Metadata)
	if err != nil {
		return err
	}

	const query = `
		INSERT INTO audit_logs (user_id, action, entity_type, entity_id, metadata, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = postgres.Executor(ctx, r.pool).Exec(
		ctx,
		query,
		event.UserID,
		event.Action,
		event.EntityType,
		event.EntityID,
		metadata,
		event.IP,
		event.UserAgent,
	)
	return err
}
