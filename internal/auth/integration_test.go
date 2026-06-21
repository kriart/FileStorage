package auth

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"file-storage-server/internal/config"
	"file-storage-server/internal/postgres"
	"file-storage-server/internal/repository"
	userpkg "file-storage-server/internal/user"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRefreshTokenIntegrationRotatesToken(t *testing.T) {
	ctx := context.Background()
	pool := openAuthIntegrationPool(t, ctx)

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	email := "refresh-it-" + suffix + "@example.test"
	service := NewService(
		userpkg.NewPostgresRepository(pool),
		NewPostgresRefreshTokenRepository(pool),
		config.AuthConfig{
			JWTSecret:       "test-secret",
			TokenTTL:        time.Minute,
			RefreshTokenTTL: time.Hour,
		},
	)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, email)
	})

	registered, err := service.Register(ctx, userpkg.RegisterUserDTO{
		Username: "refresh-it-" + suffix,
		Email:    email,
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if registered.AccessToken == "" || registered.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens")
	}

	refreshed, err := service.Refresh(ctx, registered.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshed.AccessToken == "" || refreshed.RefreshToken == "" {
		t.Fatal("expected rotated access and refresh tokens")
	}
	if refreshed.RefreshToken == registered.RefreshToken {
		t.Fatal("expected refresh token rotation")
	}

	_, err = service.Refresh(ctx, registered.RefreshToken)
	if !errors.Is(err, repository.ErrUnauthorized) {
		t.Fatalf("expected old refresh token to be rejected, got %v", err)
	}
}

func TestRefreshTokenIntegrationAllowsOnlyOneConcurrentRotation(t *testing.T) {
	ctx := context.Background()
	pool := openAuthIntegrationPool(t, ctx)

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	email := "refresh-race-" + suffix + "@example.test"
	service := NewService(
		userpkg.NewPostgresRepository(pool),
		NewPostgresRefreshTokenRepository(pool),
		config.AuthConfig{
			JWTSecret:       "test-secret",
			TokenTTL:        time.Minute,
			RefreshTokenTTL: time.Hour,
		},
	)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, email)
	})

	registered, err := service.Register(ctx, userpkg.RegisterUserDTO{
		Username: "refresh-race-" + suffix,
		Email:    email,
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, errs[index] = service.Refresh(context.Background(), registered.RefreshToken)
		}(i)
	}
	wg.Wait()

	var successes, unauthorized int
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, repository.ErrUnauthorized):
			unauthorized++
		default:
			t.Fatalf("unexpected refresh error: %v", err)
		}
	}
	if successes != 1 || unauthorized != 1 {
		t.Fatalf("expected one success and one unauthorized, got successes=%d unauthorized=%d errs=%v", successes, unauthorized, errs)
	}
}

func openAuthIntegrationPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	_ = config.LoadDotEnv("../../.env")
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set")
	}

	pool, err := postgres.OpenPool(ctx, config.PostgresConfig{
		DSN:      dsn,
		MaxConns: 4,
		MinConns: 1,
	})
	if err != nil {
		t.Skipf("postgres is not available: %v", err)
	}
	t.Cleanup(pool.Close)

	var hasRefreshTokens bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_name = 'refresh_tokens'
		)
	`).Scan(&hasRefreshTokens); err != nil {
		t.Fatalf("check refresh_tokens schema: %v", err)
	}
	if !hasRefreshTokens {
		t.Skip("refresh_tokens table is missing; run migrations first")
	}

	return pool
}
