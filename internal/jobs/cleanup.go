package jobs

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"file-storage-server/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CleanupJob struct {
	pool      *pgxpool.Pool
	rootDir   string
	interval  time.Duration
	stagedTTL time.Duration
	orphanTTL time.Duration
	logger    *slog.Logger
}

func NewCleanupJob(pool *pgxpool.Pool, rootDir string, cfg config.JobsConfig, logger *slog.Logger) *CleanupJob {
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = time.Hour
	}
	if cfg.StagedFileTTL <= 0 {
		cfg.StagedFileTTL = 24 * time.Hour
	}
	if cfg.OrphanFileTTL <= 0 {
		cfg.OrphanFileTTL = 24 * time.Hour
	}
	return &CleanupJob{
		pool:      pool,
		rootDir:   rootDir,
		interval:  cfg.CleanupInterval,
		stagedTTL: cfg.StagedFileTTL,
		orphanTTL: cfg.OrphanFileTTL,
		logger:    logger,
	}
}

func (j *CleanupJob) Start(ctx context.Context) {
	go func() {
		j.run(ctx)

		ticker := time.NewTicker(j.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				j.run(ctx)
			}
		}
	}()
}

func (j *CleanupJob) run(ctx context.Context) {
	refreshTokens, err := j.deleteExpiredRefreshTokens(ctx)
	j.logResult("cleanup refresh tokens", refreshTokens, err)

	shareLinks, err := j.deactivateExpiredShareLinks(ctx)
	j.logResult("cleanup share links", shareLinks, err)

	stagedFiles, err := j.deleteStaleStagedFiles()
	j.logResult("cleanup staged files", stagedFiles, err)

	orphanFiles, err := j.deleteOrphanStoredFiles(ctx)
	j.logResult("cleanup orphan files", orphanFiles, err)
}

func (j *CleanupJob) deleteExpiredRefreshTokens(ctx context.Context) (int64, error) {
	tag, err := j.pool.Exec(ctx, `
		DELETE FROM refresh_tokens
		WHERE expires_at < now()
			OR revoked_at < now() - interval '7 days'
	`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (j *CleanupJob) deactivateExpiredShareLinks(ctx context.Context) (int64, error) {
	tag, err := j.pool.Exec(ctx, `
		UPDATE share_links
		SET is_active = false
		WHERE is_active = true
			AND expires_at IS NOT NULL
			AND expires_at <= now()
	`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (j *CleanupJob) deleteStaleStagedFiles() (int64, error) {
	tmpDir := filepath.Join(j.rootDir, "tmp")
	cutoff := time.Now().Add(-j.stagedTTL)
	var deleted int64

	err := filepath.WalkDir(tmpDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(cutoff) {
			return nil
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := pruneEmptyParents(filepath.Dir(path), tmpDir); err != nil {
			return err
		}
		deleted++
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return deleted, nil
	}
	return deleted, err
}

func (j *CleanupJob) deleteOrphanStoredFiles(ctx context.Context) (int64, error) {
	activeFiles, err := j.activeFilePaths(ctx)
	if err != nil {
		return 0, err
	}

	filesDir := filepath.Join(j.rootDir, "files")
	cutoff := time.Now().Add(-j.orphanTTL)
	var deleted int64

	err = filepath.WalkDir(filesDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(cutoff) {
			return nil
		}
		relativePath, err := filepath.Rel(j.rootDir, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)
		if _, ok := activeFiles[relativePath]; ok {
			return nil
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := pruneEmptyParents(filepath.Dir(path), filesDir); err != nil {
			return err
		}
		deleted++
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return deleted, nil
	}
	return deleted, err
}

func (j *CleanupJob) activeFilePaths(ctx context.Context) (map[string]struct{}, error) {
	rows, err := j.pool.Query(ctx, `
		SELECT relative_path
		FROM files
		WHERE deleted_at IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]struct{})
	for rows.Next() {
		var relativePath string
		if err := rows.Scan(&relativePath); err != nil {
			return nil, err
		}
		result[filepath.ToSlash(relativePath)] = struct{}{}
	}
	return result, rows.Err()
}

func pruneEmptyParents(startDir string, stopDir string) error {
	current, err := filepath.Abs(startDir)
	if err != nil {
		return err
	}
	stop, err := filepath.Abs(stopDir)
	if err != nil {
		return err
	}

	for current != stop {
		if !isWithinDir(current, stop) {
			return nil
		}
		if err := os.Remove(current); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if errors.Is(err, os.ErrPermission) {
				return err
			}
			if !isDirectoryNotEmpty(err) {
				return err
			}
			return nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
	}
	return nil
}

func isWithinDir(path string, root string) bool {
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isDirectoryNotEmpty(err error) bool {
	return errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST)
}

func (j *CleanupJob) logResult(message string, count int64, err error) {
	if j.logger == nil {
		return
	}
	if err != nil {
		j.logger.Warn(message, "error", err)
		return
	}
	if count > 0 {
		j.logger.Info(message, "count", count)
	}
}
