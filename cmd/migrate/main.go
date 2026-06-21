package main

import (
	"database/sql"
	"errors"
	"flag"
	"log/slog"
	"os"

	"file-storage-server/internal/config"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	migrationsDir := flag.String("dir", "migrations", "migrations directory")
	flag.Parse()

	command := "up"
	if flag.NArg() > 0 {
		command = flag.Arg(0)
	}

	if err := config.LoadDotEnv(".env"); err != nil {
		logger.Error("load .env", "error", err)
		os.Exit(1)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	connConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		logger.Error("parse DATABASE_URL", "error", err)
		os.Exit(1)
	}

	db := stdlib.OpenDB(*connConfig)
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		logger.Error("set goose dialect", "error", err)
		os.Exit(1)
	}

	if err := run(command, db, *migrationsDir); err != nil {
		logger.Error("run migration", "command", command, "error", err)
		os.Exit(1)
	}

	logger.Info("migration finished", "command", command)
}

func run(command string, db *sql.DB, dir string) error {
	switch command {
	case "up":
		return goose.Up(db, dir)
	case "down":
		return goose.Down(db, dir)
	case "status":
		return goose.Status(db, dir)
	case "version":
		return goose.Version(db, dir)
	default:
		return errors.New("unsupported migration command")
	}
}
