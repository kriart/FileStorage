package profile

import (
	"context"
	"errors"
	"strings"

	"file-storage-server/internal/postgres"
	"file-storage-server/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Get(ctx context.Context, userID int64) (*Profile, error) {
	userInfo, err := r.user(ctx, userID)
	if err != nil {
		return nil, err
	}
	sessions, err := r.sessions(ctx, userID)
	if err != nil {
		return nil, err
	}
	auditEvents, err := r.audit(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &Profile{User: *userInfo, Sessions: sessions, Audit: auditEvents}, nil
}

func (r *Repository) UpdateSettings(ctx context.Context, userID int64, settings UpdateSettings) (*UserInfo, error) {
	settings.Username = strings.TrimSpace(settings.Username)
	if settings.Username == "" {
		return nil, repository.InvalidInput("Имя пользователя обязательно")
	}
	if len(settings.Username) > 64 {
		return nil, repository.InvalidInput("Имя пользователя должно быть не длиннее 64 символов")
	}

	const query = `
		UPDATE users
		SET username = $2, date_of_birth = $3, updated_at = now()
		WHERE id = $1
		RETURNING id, username, email, role, avatar_path, date_of_birth, theme, language, created_at
	`
	userInfo := new(UserInfo)
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, userID, settings.Username, settings.DateOfBirth).Scan(
		&userInfo.ID,
		&userInfo.Username,
		&userInfo.Email,
		&userInfo.Role,
		&userInfo.AvatarPath,
		&userInfo.DateOfBirth,
		&userInfo.Theme,
		&userInfo.Language,
		&userInfo.CreatedAt,
	)
	if err != nil {
		return nil, mapProfileError(err)
	}
	return userInfo, nil
}

func (r *Repository) UpdatePreferences(ctx context.Context, userID int64, preferences UpdatePreferences) (*UserInfo, error) {
	preferences.Theme = normalizeTheme(preferences.Theme)
	preferences.Language = normalizeLanguage(preferences.Language)

	const query = `
		UPDATE users
		SET theme = $2, language = $3, updated_at = now()
		WHERE id = $1
		RETURNING id, username, email, role, avatar_path, date_of_birth, theme, language, created_at
	`
	userInfo := new(UserInfo)
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, userID, preferences.Theme, preferences.Language).Scan(
		&userInfo.ID,
		&userInfo.Username,
		&userInfo.Email,
		&userInfo.Role,
		&userInfo.AvatarPath,
		&userInfo.DateOfBirth,
		&userInfo.Theme,
		&userInfo.Language,
		&userInfo.CreatedAt,
	)
	if err != nil {
		return nil, mapProfileError(err)
	}
	return userInfo, nil
}

func (r *Repository) UpdateAvatarPath(ctx context.Context, userID int64, avatarPath string) (*UserInfo, error) {
	const query = `
		UPDATE users
		SET avatar_path = $2, updated_at = now()
		WHERE id = $1
		RETURNING id, username, email, role, avatar_path, date_of_birth, theme, language, created_at
	`
	userInfo := new(UserInfo)
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, userID, avatarPath).Scan(
		&userInfo.ID,
		&userInfo.Username,
		&userInfo.Email,
		&userInfo.Role,
		&userInfo.AvatarPath,
		&userInfo.DateOfBirth,
		&userInfo.Theme,
		&userInfo.Language,
		&userInfo.CreatedAt,
	)
	if err != nil {
		return nil, mapProfileError(err)
	}
	return userInfo, nil
}

func (r *Repository) RevokeSession(ctx context.Context, userID int64, sessionID int64) error {
	const query = `
		UPDATE refresh_tokens
		SET revoked_at = COALESCE(revoked_at, now()), rotated_at = now()
		WHERE id = $1 AND user_id = $2
	`
	tag, err := postgres.Executor(ctx, r.pool).Exec(ctx, query, sessionID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *Repository) user(ctx context.Context, userID int64) (*UserInfo, error) {
	const query = `
		SELECT id, username, email, role, avatar_path, date_of_birth, theme, language, created_at
		FROM users
		WHERE id = $1
	`
	userInfo := new(UserInfo)
	err := postgres.Executor(ctx, r.pool).QueryRow(ctx, query, userID).Scan(
		&userInfo.ID,
		&userInfo.Username,
		&userInfo.Email,
		&userInfo.Role,
		&userInfo.AvatarPath,
		&userInfo.DateOfBirth,
		&userInfo.Theme,
		&userInfo.Language,
		&userInfo.CreatedAt,
	)
	if err != nil {
		return nil, mapProfileError(err)
	}
	return userInfo, nil
}

func normalizeTheme(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "dark" {
		return "dark"
	}
	return "light"
}

func normalizeLanguage(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "en" {
		return "en"
	}
	return "ru"
}

func (r *Repository) sessions(ctx context.Context, userID int64) ([]Session, error) {
	const query = `
		SELECT id, created_at, expires_at, revoked_at, revoked_at IS NULL AND expires_at > now()
		FROM refresh_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 20
	`
	rows, err := postgres.Executor(ctx, r.pool).Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Session, 0)
	for rows.Next() {
		row := Session{}
		if err := rows.Scan(&row.ID, &row.CreatedAt, &row.ExpiresAt, &row.RevokedAt, &row.Active); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *Repository) audit(ctx context.Context, userID int64) ([]AuditEvent, error) {
	const query = `
		SELECT id, action, entity_type, entity_id, COALESCE(ip, ''), created_at
		FROM audit_logs
		WHERE user_id = $1
		ORDER BY id DESC
		LIMIT 30
	`
	rows, err := postgres.Executor(ctx, r.pool).Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]AuditEvent, 0)
	for rows.Next() {
		row := AuditEvent{}
		if err := rows.Scan(&row.ID, &row.Action, &row.EntityType, &row.EntityID, &row.IP, &row.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func mapProfileError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return repository.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return repository.Conflict("Такое имя пользователя уже занято")
	}
	return err
}
