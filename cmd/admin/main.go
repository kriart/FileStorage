package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"strings"

	"file-storage-server/internal/config"
	"file-storage-server/internal/domain"
	"file-storage-server/internal/postgres"
	"file-storage-server/internal/repository"
	"file-storage-server/internal/user"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	email := flag.String("email", "", "admin email")
	username := flag.String("username", "", "admin username")
	password := flag.String("password", "", "admin password")
	flag.Parse()

	if err := config.LoadDotEnv(".env"); err != nil {
		logger.Error("load .env", "error", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	*email = strings.ToLower(strings.TrimSpace(*email))
	*username = strings.TrimSpace(*username)
	if *email == "" || *username == "" || *password == "" {
		logger.Error("email, username and password are required")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := postgres.OpenPool(ctx, cfg.Postgres)
	if err != nil {
		logger.Error("open postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	users := user.NewPostgresRepository(pool)
	existing, err := users.GetByEmail(ctx, *email)
	if err == nil {
		if existing.Role != domain.UserRoleAdmin {
			if _, err := users.UpdateRole(ctx, user.UpdateUserRoleParams{UserID: existing.ID, Role: domain.UserRoleAdmin}); err != nil {
				logger.Error("promote admin", "error", err)
				os.Exit(1)
			}
		}
		logger.Info("admin user ready", "email", existing.Email, "id", existing.ID)
		return
	}
	if !errors.Is(err, repository.ErrNotFound) {
		logger.Error("lookup user", "error", err)
		os.Exit(1)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("hash password", "error", err)
		os.Exit(1)
	}

	created, err := users.Create(ctx, user.CreateUserParams{
		Username:     *username,
		Email:        *email,
		PasswordHash: string(hash),
		Role:         domain.UserRoleAdmin,
	})
	if err != nil {
		logger.Error("create admin", "error", err)
		os.Exit(1)
	}

	logger.Info("admin user created", "email", created.Email, "id", created.ID)
}
