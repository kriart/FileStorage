package share

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"file-storage-server/internal/config"
	"file-storage-server/internal/domain"
	"file-storage-server/internal/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestShareLinksIntegrationCreateAndListExposeURLs(t *testing.T) {
	ctx := context.Background()
	pool := openIntegrationPool(t, ctx)
	fixture := createShareFixture(t, ctx, pool)
	service := NewService(NewPostgresRepository(pool), nil, nil, nil, allowSharePermission{})

	created, err := service.Create(ctx, fixture.userID, fixture.fileID, CreateShareLinkDTO{
		AccessType: domain.ShareAccessRead,
	}, "http://storage.test")
	if err != nil {
		t.Fatalf("create share link: %v", err)
	}
	if !strings.HasPrefix(created.URL, "http://storage.test/api/public/share/") {
		t.Fatalf("expected public share URL, got %q", created.URL)
	}

	links, err := service.List(ctx, fixture.userID, fixture.fileID, "http://storage.test")
	if err != nil {
		t.Fatalf("list share links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected one link, got %d", len(links))
	}
	if links[0].URL != created.URL {
		t.Fatalf("expected listed URL %q, got %q", created.URL, links[0].URL)
	}

	var token *string
	if err := pool.QueryRow(ctx, `SELECT token FROM share_links WHERE id = $1`, created.ID).Scan(&token); err != nil {
		t.Fatalf("read persisted token: %v", err)
	}
	if token == nil || *token == "" {
		t.Fatal("expected share token to be persisted")
	}
}

func TestShareLinksIntegrationListBackfillsLegacyTokens(t *testing.T) {
	ctx := context.Background()
	pool := openIntegrationPool(t, ctx)
	fixture := createShareFixture(t, ctx, pool)
	service := NewService(NewPostgresRepository(pool), nil, nil, nil, allowSharePermission{})

	var linkID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO share_links (file_id, token_hash, access_type, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, fixture.fileID, hashToken("legacy-token"), domain.ShareAccessRead, fixture.userID).Scan(&linkID); err != nil {
		t.Fatalf("insert legacy share link: %v", err)
	}

	links, err := service.List(ctx, fixture.userID, fixture.fileID, "http://storage.test")
	if err != nil {
		t.Fatalf("list share links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected one link, got %d", len(links))
	}
	if !strings.HasPrefix(links[0].URL, "http://storage.test/api/public/share/") {
		t.Fatalf("expected backfilled public URL, got %q", links[0].URL)
	}

	var token string
	var tokenHash string
	if err := pool.QueryRow(ctx, `SELECT token, token_hash FROM share_links WHERE id = $1`, linkID).Scan(&token, &tokenHash); err != nil {
		t.Fatalf("read backfilled token: %v", err)
	}
	if token == "" {
		t.Fatal("expected legacy link token to be backfilled")
	}
	if tokenHash != hashToken(token) {
		t.Fatal("expected token hash to match backfilled token")
	}
}

type shareFixture struct {
	userID    int64
	storageID int64
	fileID    int64
}

func openIntegrationPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
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

	var hasTokenColumn bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_name = 'share_links'
				AND column_name = 'token'
		)
	`).Scan(&hasTokenColumn); err != nil {
		t.Fatalf("check share_links schema: %v", err)
	}
	if !hasTokenColumn {
		t.Skip("share_links.token column is missing; run migrations first")
	}

	return pool
}

func createShareFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) shareFixture {
	t.Helper()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	fixture := shareFixture{}

	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, "share-it-"+suffix, "share-it-"+suffix+"@example.test", "hash", domain.UserRoleUser).Scan(&fixture.userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		INSERT INTO storages (name, type, visibility, max_file_size, max_storage_size)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, "share-it-"+suffix, domain.StorageTypePersonal, domain.StorageVisibilityPrivate, int64(1024), int64(1024*1024)).Scan(&fixture.storageID); err != nil {
		t.Fatalf("insert storage: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_accesses (storage_id, user_id, access_level)
		VALUES ($1, $2, $3)
	`, fixture.storageID, fixture.userID, domain.StorageAccessOwner); err != nil {
		t.Fatalf("insert storage access: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		INSERT INTO files (storage_id, owner_id, original_name, stored_name, relative_path, mime_type, size, checksum)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, fixture.storageID, fixture.userID, "share-it.txt", "share-it-"+suffix+".txt", "integration/share-it-"+suffix+".txt", "text/plain", int64(12), "checksum-"+suffix).Scan(&fixture.fileID); err != nil {
		t.Fatalf("insert file: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM storages WHERE id = $1`, fixture.storageID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, fixture.userID)
	})

	return fixture
}

type allowSharePermission struct{}

func (allowSharePermission) CanShareFile(context.Context, int64, int64) (bool, error) {
	return true, nil
}
