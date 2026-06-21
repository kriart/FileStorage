package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"file-storage-server/internal/access"
	"file-storage-server/internal/audit"
	"file-storage-server/internal/auth"
	"file-storage-server/internal/config"
	"file-storage-server/internal/domain"
	"file-storage-server/internal/file"
	"file-storage-server/internal/filesystem"
	"file-storage-server/internal/folder"
	"file-storage-server/internal/middleware"
	"file-storage-server/internal/postgres"
	"file-storage-server/internal/share"
	"file-storage-server/internal/storage"
	"file-storage-server/internal/user"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAPIIntegrationUploadDownloadAndShare(t *testing.T) {
	ctx := context.Background()
	pool := openAPIIntegrationPool(t, ctx)
	server := newAPITestServer(t, pool)
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	client := server.Client()
	client.Jar = jar

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	email := "api-it-" + suffix + "@example.test"
	storageName := "api-it-" + suffix
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM storages WHERE name = $1`, storageName)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, email)
	})

	status, body := postJSON(t, client, server.URL+"/api/auth/register", map[string]any{
		"username": "api-it-" + suffix,
		"email":    email,
		"password": "password123",
	})
	if status != http.StatusCreated {
		t.Fatalf("register status=%d body=%s", status, body)
	}

	status, body = postJSON(t, client, server.URL+"/api/storages/", map[string]any{
		"name":           storageName,
		"type":           domain.StorageTypePersonal,
		"visibility":     domain.StorageVisibilityPrivate,
		"maxFileSize":    1024,
		"maxStorageSize": 4096,
	})
	if status != http.StatusCreated {
		t.Fatalf("create storage status=%d body=%s", status, body)
	}
	var createdStorage storage.StorageDTO
	if err := json.Unmarshal(body, &createdStorage); err != nil {
		t.Fatalf("decode storage: %v", err)
	}

	status, body = uploadMultipart(t, client, server.URL+"/api/storages/"+strconv.FormatInt(createdStorage.ID, 10)+"/files", "hello.txt", "hello from integration")
	if status != http.StatusCreated {
		t.Fatalf("upload status=%d body=%s", status, body)
	}
	var uploaded file.FileDTO
	if err := json.Unmarshal(body, &uploaded); err != nil {
		t.Fatalf("decode uploaded file: %v", err)
	}

	resp, err := client.Get(server.URL + "/api/files/" + strconv.FormatInt(uploaded.ID, 10) + "/download")
	if err != nil {
		t.Fatalf("download file: %v", err)
	}
	downloadBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(downloadBody) != "hello from integration" {
		t.Fatalf("download status=%d body=%q", resp.StatusCode, string(downloadBody))
	}

	status, body = postJSON(t, client, server.URL+"/api/files/"+strconv.FormatInt(uploaded.ID, 10)+"/links", map[string]any{
		"accessType": domain.ShareAccessRead,
	})
	if status != http.StatusCreated {
		t.Fatalf("create share status=%d body=%s", status, body)
	}
	var createdLink share.ShareLinkDTO
	if err := json.Unmarshal(body, &createdLink); err != nil {
		t.Fatalf("decode share link: %v", err)
	}
	if createdLink.URL == "" {
		t.Fatal("expected share URL")
	}

	resp, err = client.Get(createdLink.URL)
	if err != nil {
		t.Fatalf("public download: %v", err)
	}
	publicBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(publicBody) != "hello from integration" {
		t.Fatalf("public download status=%d body=%q", resp.StatusCode, string(publicBody))
	}

	var auditCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM audit_logs
		WHERE action IN ('auth.register', 'storage.create', 'file.upload', 'file.download', 'share.create', 'share.public_download')
	`).Scan(&auditCount); err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if auditCount < 6 {
		t.Fatalf("expected audit logs for integration flow, got %d", auditCount)
	}
}

func newAPITestServer(t *testing.T, pool *pgxpool.Pool) *httptest.Server {
	t.Helper()

	userRepository := user.NewPostgresRepository(pool)
	storageRepository := storage.NewPostgresRepository(pool)
	fileRepository := file.NewPostgresRepository(pool)
	folderRepository := folder.NewPostgresRepository(pool)
	accessRepository := access.NewPostgresRepository(pool)
	shareRepository := share.NewPostgresRepository(pool)
	refreshTokenRepository := auth.NewPostgresRefreshTokenRepository(pool)
	auditService := audit.NewService(audit.NewPostgresRepository(pool), nil)
	fileStorage := filesystem.NewLocalStorage(t.TempDir())
	unitOfWork := postgres.NewUnitOfWork(pool)

	authService := auth.NewService(userRepository, refreshTokenRepository, config.AuthConfig{
		JWTSecret:       "integration-secret",
		TokenTTL:        time.Minute,
		RefreshTokenTTL: time.Hour,
	})
	permissionService := access.NewPermissionService(userRepository, storageRepository, fileRepository, accessRepository)
	storageService := storage.NewService(storageRepository, permissionService)
	storageAccessService := access.NewStorageAccessService(accessRepository, userRepository, folderRepository, permissionService)
	fileService := file.NewService(fileRepository, storageRepository, folderRepository, fileStorage, unitOfWork, permissionService)
	shareService := share.NewService(shareRepository, fileRepository, fileService, fileStorage, permissionService)

	router := chi.NewRouter()
	RegisterRoutes(router, RouterDeps{
		AuthMiddleware: middleware.Auth(authService),
		Auth:           NewAuthHandler(authService, auditService),
		Storages:       NewStorageHandler(storageService, auditService),
		Accesses:       NewAccessHandler(storageAccessService, auditService),
		Files:          NewFileHandler(fileService, auditService),
		Shares:         NewShareHandler(shareService, auditService),
	})
	return httptest.NewServer(router)
}

func openAPIIntegrationPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
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

	for _, table := range []string{"refresh_tokens", "audit_logs", "storage_type_rules"} {
		var exists bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.tables
				WHERE table_name = $1
			)
		`, table).Scan(&exists); err != nil {
			t.Fatalf("check schema: %v", err)
		}
		if !exists {
			t.Skipf("%s table is missing; run migrations first", table)
		}
	}

	return pool
}

func postJSON(t *testing.T, client *http.Client, url string, payload any) (int, []byte) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post JSON: %v", err)
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, responseBody
}

func uploadMultipart(t *testing.T, client *http.Client, url string, filename string, content string) (int, []byte) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := io.Copy(part, strings.NewReader(content)); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		t.Fatalf("create upload request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("upload file: %v", err)
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, responseBody
}
