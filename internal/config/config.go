package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Files    FilesConfig
	Auth     AuthConfig
	Jobs     JobsConfig
}

type HTTPConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type PostgresConfig struct {
	DSN             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
}

type FilesConfig struct {
	RootDir string
}

type AuthConfig struct {
	JWTSecret       string
	TokenTTL        time.Duration
	RefreshTokenTTL time.Duration
}

type JobsConfig struct {
	CleanupInterval time.Duration
	StagedFileTTL   time.Duration
	OrphanFileTTL   time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTP: HTTPConfig{
			Addr:         getEnv("HTTP_ADDR", ":8080"),
			ReadTimeout:  getDurationEnv("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout: getDurationEnv("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:  getDurationEnv("HTTP_IDLE_TIMEOUT", 60*time.Second),
		},
		Postgres: PostgresConfig{
			DSN:             os.Getenv("DATABASE_URL"),
			MaxConns:        int32(getIntEnv("PG_MAX_CONNS", 10)),
			MinConns:        int32(getIntEnv("PG_MIN_CONNS", 1)),
			MaxConnLifetime: getDurationEnv("PG_MAX_CONN_LIFETIME", time.Hour),
		},
		Files: FilesConfig{
			RootDir: getEnv("FILE_STORAGE_ROOT", "data/file-storage"),
		},
		Auth: AuthConfig{
			JWTSecret:       os.Getenv("JWT_SECRET"),
			TokenTTL:        getDurationEnv("ACCESS_TOKEN_TTL", getDurationEnv("JWT_TTL", 15*time.Minute)),
			RefreshTokenTTL: getDurationEnv("REFRESH_TOKEN_TTL", 30*24*time.Hour),
		},
		Jobs: JobsConfig{
			CleanupInterval: getDurationEnv("CLEANUP_INTERVAL", time.Hour),
			StagedFileTTL:   getDurationEnv("STAGED_FILE_TTL", 24*time.Hour),
			OrphanFileTTL:   getDurationEnv("ORPHAN_FILE_TTL", 24*time.Hour),
		},
	}

	if cfg.Postgres.DSN == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.Auth.JWTSecret == "" {
		return Config{}, errors.New("JWT_SECRET is required")
	}

	return cfg, nil
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getIntEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
