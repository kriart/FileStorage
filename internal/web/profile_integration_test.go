package web

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"file-storage-server/internal/access"
	"file-storage-server/internal/admin"
	"file-storage-server/internal/audit"
	"file-storage-server/internal/auth"
	"file-storage-server/internal/config"
	"file-storage-server/internal/file"
	"file-storage-server/internal/filesystem"
	"file-storage-server/internal/folder"
	"file-storage-server/internal/metrics"
	"file-storage-server/internal/middleware"
	"file-storage-server/internal/postgres"
	"file-storage-server/internal/profile"
	"file-storage-server/internal/share"
	"file-storage-server/internal/storage"
	"file-storage-server/internal/user"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProfilePreferencesAndAvatarIntegration(t *testing.T) {
	ctx := context.Background()
	pool := openWebIntegrationPool(t, ctx)
	server := newWebTestServer(t, pool, t.TempDir())
	defer server.Close()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	email := "web-profile-" + suffix + "@example.test"
	username := "web-profile-" + suffix
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, email)
	})

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := server.Client()
	client.Jar = jar

	status, body := postForm(t, client, server.URL+"/register", url.Values{
		"username": {username},
		"email":    {email},
		"password": {"password123"},
	})
	if status != http.StatusOK {
		t.Fatalf("register status=%d body=%s", status, body)
	}

	status, body = postForm(t, client, server.URL+"/profile/preferences", url.Values{
		"theme":    {"dark"},
		"language": {"en"},
		"returnTo": {"/profile"},
	})
	if status != http.StatusOK {
		t.Fatalf("preferences status=%d body=%s", status, body)
	}
	if !strings.Contains(body, `<body data-theme="dark">`) || !strings.Contains(body, `<html lang="en">`) {
		t.Fatalf("expected dark/en profile page")
	}

	status, body = postForm(t, client, server.URL+"/profile", url.Values{
		"username":    {username},
		"dateOfBirth": {"1990-01-02"},
	})
	if status != http.StatusOK {
		t.Fatalf("profile update status=%d body=%s", status, body)
	}
	if !strings.Contains(body, "02.01.1990") {
		t.Fatalf("expected saved birth date")
	}

	status, body = uploadAvatar(t, client, server.URL+"/profile/avatar")
	if status != http.StatusOK {
		t.Fatalf("avatar upload status=%d body=%s", status, body)
	}

	resp, err := client.Get(server.URL + "/profile/avatar")
	if err != nil {
		t.Fatalf("get avatar: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("avatar status=%d", resp.StatusCode)
	}
	if contentType := resp.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "image/png") {
		t.Fatalf("expected image/png avatar, got %q", contentType)
	}
}

func newWebTestServer(t *testing.T, pool *pgxpool.Pool, fileRoot string) *httptest.Server {
	t.Helper()

	userRepository := user.NewPostgresRepository(pool)
	storageRepository := storage.NewPostgresRepository(pool)
	fileRepository := file.NewPostgresRepository(pool)
	folderRepository := folder.NewPostgresRepository(pool)
	accessRepository := access.NewPostgresRepository(pool)
	adminRepository := admin.NewRepository(pool)
	adminService := admin.NewService(adminRepository)
	profileRepository := profile.NewRepository(pool)
	profileService := profile.NewService(profileRepository)
	shareRepository := share.NewPostgresRepository(pool)
	refreshTokenRepository := auth.NewPostgresRefreshTokenRepository(pool)
	auditRepository := audit.NewPostgresRepository(pool)
	fileStorage := filesystem.NewLocalStorage(fileRoot)
	unitOfWork := postgres.NewUnitOfWork(pool)

	auditService := audit.NewService(auditRepository, nil)
	authService := auth.NewService(userRepository, refreshTokenRepository, config.AuthConfig{
		JWTSecret:       "web-test-secret",
		TokenTTL:        time.Minute,
		RefreshTokenTTL: time.Hour,
	})
	permissionService := access.NewPermissionService(userRepository, storageRepository, fileRepository, accessRepository)
	storageService := storage.NewService(storageRepository, permissionService)
	storageAccessService := access.NewStorageAccessService(accessRepository, userRepository, folderRepository, permissionService)
	folderService := folder.NewService(folderRepository, storageRepository, unitOfWork, permissionService)
	fileService := file.NewService(fileRepository, storageRepository, folderRepository, fileStorage, unitOfWork, permissionService)
	shareService := share.NewService(shareRepository, fileRepository, fileService, fileStorage, permissionService)
	metricsRegistry := metrics.NewRegistry()

	handler, err := NewHandler(
		authService,
		storageService,
		fileService,
		folderService,
		storageAccessService,
		adminService,
		metricsRegistry,
		profileService,
		shareService,
		auditService,
		fileRoot,
		"../../templates/*.html",
	)
	if err != nil {
		t.Fatalf("new web handler: %v", err)
	}

	router := chi.NewRouter()
	handler.Routes(router, middleware.WebAuth(authService, "/login"), nil)
	return httptest.NewServer(router)
}

func openWebIntegrationPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
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

	var hasUsers bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_name = 'users'
		)
	`).Scan(&hasUsers); err != nil {
		t.Fatalf("check schema: %v", err)
	}
	if !hasUsers {
		t.Skip("users table is missing; run migrations first")
	}

	return pool
}

func postForm(t *testing.T, client *http.Client, target string, values url.Values) (int, string) {
	t.Helper()

	resp, err := client.PostForm(target, values)
	if err != nil {
		t.Fatalf("post form %s: %v", target, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return resp.StatusCode, string(body)
}

func uploadAvatar(t *testing.T, client *http.Client, target string) (int, string) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("avatar", "avatar.png")
	if err != nil {
		t.Fatalf("create avatar part: %v", err)
	}
	if _, err := part.Write([]byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x04, 0x00, 0x00, 0x00, 0xb5, 0x1c, 0x0c,
		0x02, 0x00, 0x00, 0x00, 0x0b, 0x49, 0x44, 0x41,
		0x54, 0x78, 0xda, 0x63, 0xfc, 0xff, 0x1f, 0x00,
		0x03, 0x03, 0x02, 0x00, 0xef, 0xbf, 0xa7, 0xdb,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44,
		0xae, 0x42, 0x60, 0x82,
	}); err != nil {
		t.Fatalf("write avatar: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, target, &body)
	if err != nil {
		t.Fatalf("new avatar request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("upload avatar: %v", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read avatar response: %v", err)
	}
	return resp.StatusCode, string(responseBody)
}
