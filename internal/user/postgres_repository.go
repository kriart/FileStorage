package user

import (
	"context"
	"errors"

	"file-storage-server/internal/domain"
	"file-storage-server/internal/postgres"
	"file-storage-server/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, params CreateUserParams) (*domain.User, error) {
	const query = `
		INSERT INTO users (username, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, username, email, password_hash, role, avatar_path, date_of_birth, theme, language, created_at, updated_at
	`

	user := new(domain.User)
	err := postgres.Executor(ctx, r.pool).QueryRow(
		ctx,
		query,
		params.Username,
		params.Email,
		params.PasswordHash,
		params.Role,
	).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.AvatarPath,
		&user.DateOfBirth,
		&user.Theme,
		&user.Language,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, mapUserError(err)
	}
	return user, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	return r.getOne(ctx, `WHERE id = $1`, id)
}

func (r *PostgresRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.getOne(ctx, `WHERE email = $1`, email)
}

func (r *PostgresRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return r.getOne(ctx, `WHERE username = $1`, username)
}

func (r *PostgresRepository) List(ctx context.Context) ([]domain.User, error) {
	const query = `
		SELECT id, username, email, password_hash, role, avatar_path, date_of_birth, theme, language, created_at, updated_at
		FROM users
		ORDER BY id
	`

	rows, err := postgres.Executor(ctx, r.pool).Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]domain.User, 0)
	for rows.Next() {
		user := domain.User{}
		if err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.PasswordHash,
			&user.Role,
			&user.AvatarPath,
			&user.DateOfBirth,
			&user.Theme,
			&user.Language,
			&user.CreatedAt,
			&user.UpdatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *PostgresRepository) UpdateRole(ctx context.Context, params UpdateUserRoleParams) (*domain.User, error) {
	const query = `
		UPDATE users
		SET role = $2, updated_at = now()
		WHERE id = $1
		RETURNING id, username, email, password_hash, role, avatar_path, date_of_birth, theme, language, created_at, updated_at
	`

	user := new(domain.User)
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, params.UserID, params.Role).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.AvatarPath,
		&user.DateOfBirth,
		&user.Theme,
		&user.Language,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, mapUserError(err)
	}
	return user, nil
}

func (r *PostgresRepository) getOne(ctx context.Context, where string, arg any) (*domain.User, error) {
	query := `
		SELECT id, username, email, password_hash, role, avatar_path, date_of_birth, theme, language, created_at, updated_at
		FROM users
	` + where

	user := new(domain.User)
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, arg).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.AvatarPath,
		&user.DateOfBirth,
		&user.Theme,
		&user.Language,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, mapUserError(err)
	}
	return user, nil
}

func mapUserError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return repository.ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return repository.ErrConflict
	}

	return err
}
