package file

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"file-storage-server/internal/domain"
	"file-storage-server/internal/filesystem"
	"file-storage-server/internal/folder"
	"file-storage-server/internal/repository"
	storagepkg "file-storage-server/internal/storage"

	"github.com/google/uuid"
)

type PermissionChecker interface {
	CanViewStorage(ctx context.Context, userID int64, storageID int64) (bool, error)
	CanUploadToStorage(ctx context.Context, userID int64, storageID int64) (bool, error)
	CanDownloadFile(ctx context.Context, userID int64, fileID int64) (bool, error)
	CanDeleteFile(ctx context.Context, userID int64, fileID int64) (bool, error)
	CanEditFile(ctx context.Context, userID int64, fileID int64) (bool, error)
}

type Service struct {
	files      Repository
	storages   storagepkg.Repository
	folders    folder.Repository
	fs         filesystem.Storage
	uow        repository.UnitOfWork
	permission PermissionChecker
}

type Download struct {
	File   *domain.File
	Reader io.ReadCloser
}

func NewService(
	files Repository,
	storages storagepkg.Repository,
	folders folder.Repository,
	fs filesystem.Storage,
	uow repository.UnitOfWork,
	permission PermissionChecker,
) *Service {
	return &Service{
		files:      files,
		storages:   storages,
		folders:    folders,
		fs:         fs,
		uow:        uow,
		permission: permission,
	}
}

func (s *Service) Upload(ctx context.Context, actorID int64, storageID int64, folderID *int64, originalName string, reader io.Reader) (*FileDTO, error) {
	if reader == nil {
		return nil, repository.InvalidInput("Файл обязателен")
	}

	allowed, err := s.permission.CanUploadToStorage(ctx, actorID, storageID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}

	if err := s.validateFolder(ctx, storageID, folderID); err != nil {
		return nil, err
	}

	staged, mimeType, name, storage, err := s.stageValidated(ctx, storageID, originalName, reader)
	if err != nil {
		return nil, err
	}
	var saved *filesystem.SavedFile
	shouldCleanup := true
	defer func() {
		if shouldCleanup {
			_ = s.fs.DeleteStaged(context.Background(), staged)
			if saved != nil {
				_ = s.fs.Delete(context.Background(), saved.RelativePath)
			}
		}
	}()

	var created *domain.File
	err = s.uow.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		saved, err = s.fs.Commit(ctx, staged)
		if err != nil {
			return err
		}
		if storage.UsedSize+saved.Size > storage.MaxStorageSize {
			return repository.LimitExceeded("Файл не загружен: в хранилище недостаточно свободного места")
		}
		if _, err := s.storages.AdjustUsedSize(ctx, storagepkg.AdjustStorageSizeParams{
			StorageID: storageID,
			Delta:     saved.Size,
		}); err != nil {
			if errors.Is(err, repository.ErrLimitExceeded) {
				return repository.LimitExceeded("Файл не загружен: в хранилище недостаточно свободного места")
			}
			return err
		}

		file, err := s.files.Create(ctx, CreateFileParams{
			StorageID:    storageID,
			FolderID:     folderID,
			OwnerID:      actorID,
			OriginalName: name,
			StoredName:   saved.StoredName,
			RelativePath: saved.RelativePath,
			MimeType:     mimeType,
			Size:         saved.Size,
			Checksum:     saved.Checksum,
		})
		if err != nil {
			return err
		}

		created = file
		return nil
	})
	if err != nil {
		return nil, err
	}

	shouldCleanup = false
	return toDTO(created), nil
}

func (s *Service) Replace(ctx context.Context, actorID int64, fileID int64, originalName string, reader io.Reader) (*FileDTO, error) {
	if reader == nil {
		return nil, repository.InvalidInput("Файл обязателен")
	}

	allowed, err := s.permission.CanEditFile(ctx, actorID, fileID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}

	return s.replaceContent(ctx, fileID, originalName, reader, nil)
}

func (s *Service) ReplaceFromShare(ctx context.Context, fileID int64, originalName string, reader io.Reader) (*FileDTO, error) {
	if reader == nil {
		return nil, repository.InvalidInput("Файл обязателен")
	}
	return s.replaceContent(ctx, fileID, originalName, reader, nil)
}

func (s *Service) ReplaceFromShareWithHook(
	ctx context.Context,
	fileID int64,
	originalName string,
	reader io.Reader,
	beforeCommit func(context.Context) error,
) (*FileDTO, error) {
	if reader == nil {
		return nil, repository.InvalidInput("Файл обязателен")
	}
	return s.replaceContent(ctx, fileID, originalName, reader, beforeCommit)
}

func (s *Service) Rename(ctx context.Context, actorID int64, fileID int64, dto RenameFileDTO) (*FileDTO, error) {
	allowed, err := s.permission.CanEditFile(ctx, actorID, fileID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}
	return s.renameContent(ctx, fileID, dto.Name, nil)
}

func (s *Service) RenameFromShareWithHook(ctx context.Context, fileID int64, name string, beforeCommit func(context.Context) error) (*FileDTO, error) {
	return s.renameContent(ctx, fileID, name, beforeCommit)
}

func (s *Service) DeleteFromShareWithHook(ctx context.Context, fileID int64, beforeCommit func(context.Context) error) error {
	return s.deleteContent(ctx, fileID, 0, beforeCommit)
}

func (s *Service) replaceContent(
	ctx context.Context,
	fileID int64,
	originalName string,
	reader io.Reader,
	beforeCommit func(context.Context) error,
) (*FileDTO, error) {
	currentFile, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if currentFile.DeletedAt != nil {
		return nil, repository.ErrNotFound
	}

	staged, mimeType, name, storage, err := s.stageValidated(ctx, currentFile.StorageID, originalName, reader)
	if err != nil {
		return nil, err
	}
	var saved *filesystem.SavedFile
	shouldCleanup := true
	defer func() {
		if shouldCleanup {
			_ = s.fs.DeleteStaged(context.Background(), staged)
			if saved != nil {
				_ = s.fs.Delete(context.Background(), saved.RelativePath)
			}
		}
	}()

	var updated *domain.File
	err = s.uow.WithinTx(ctx, func(ctx context.Context) error {
		if beforeCommit != nil {
			if err := beforeCommit(ctx); err != nil {
				return err
			}
		}
		var err error
		saved, err = s.fs.Commit(ctx, staged)
		if err != nil {
			return err
		}
		if storage.UsedSize-currentFile.Size+saved.Size > storage.MaxStorageSize {
			return repository.LimitExceeded("Файл не заменен: в хранилище недостаточно свободного места")
		}

		if _, err := s.storages.AdjustUsedSize(ctx, storagepkg.AdjustStorageSizeParams{
			StorageID: currentFile.StorageID,
			Delta:     saved.Size - currentFile.Size,
		}); err != nil {
			if errors.Is(err, repository.ErrLimitExceeded) {
				return repository.LimitExceeded("Файл не заменен: в хранилище недостаточно свободного места")
			}
			return err
		}

		file, err := s.files.UpdateContent(ctx, UpdateFileContentParams{
			FileID:       fileID,
			OriginalName: name,
			StoredName:   saved.StoredName,
			RelativePath: saved.RelativePath,
			MimeType:     mimeType,
			Size:         saved.Size,
			Checksum:     saved.Checksum,
		})
		if err != nil {
			return err
		}

		updated = file
		return nil
	})
	if err != nil {
		return nil, err
	}

	shouldCleanup = false
	_ = s.fs.Delete(context.Background(), currentFile.RelativePath)
	return toDTO(updated), nil
}

func (s *Service) renameContent(ctx context.Context, fileID int64, name string, beforeCommit func(context.Context) error) (*FileDTO, error) {
	currentFile, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if currentFile.DeletedAt != nil {
		return nil, repository.ErrNotFound
	}

	nextName, err := validateRenamedFileName(currentFile.OriginalName, name)
	if err != nil {
		return nil, err
	}

	var renamed *domain.File
	err = s.uow.WithinTx(ctx, func(ctx context.Context) error {
		if beforeCommit != nil {
			if err := beforeCommit(ctx); err != nil {
				return err
			}
		}

		fileModel, err := s.files.Rename(ctx, RenameFileParams{FileID: fileID, OriginalName: nextName})
		if err != nil {
			return err
		}
		renamed = fileModel
		return nil
	})
	if err != nil {
		return nil, err
	}
	return toDTO(renamed), nil
}

func (s *Service) List(ctx context.Context, actorID int64, storageID int64, folderID *int64) ([]FileDTO, error) {
	page, err := s.ListPage(ctx, actorID, ListFilesFilter{StorageID: storageID, FolderID: folderID})
	if err != nil {
		return nil, err
	}
	return page.Files, nil
}

func (s *Service) ListPage(ctx context.Context, actorID int64, filter ListFilesFilter) (*ListFilesPage, error) {
	allowed, err := s.permission.CanViewStorage(ctx, actorID, filter.StorageID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}

	if err := s.validateFolder(ctx, filter.StorageID, filter.FolderID); err != nil {
		return nil, err
	}
	filter = normalizeListFilter(filter)

	files, err := s.files.ListByStorage(ctx, filter)
	if err != nil {
		return nil, err
	}
	total, err := s.files.CountByStorage(ctx, filter)
	if err != nil {
		return nil, err
	}

	result := make([]FileDTO, 0, len(files))
	for i := range files {
		result = append(result, *toDTO(&files[i]))
	}
	return &ListFilesPage{Files: result, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func normalizeListFilter(filter ListFilesFilter) ListFilesFilter {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	filter.Search = strings.TrimSpace(filter.Search)
	return filter
}

func (s *Service) Get(ctx context.Context, actorID int64, fileID int64) (*FileDTO, error) {
	allowed, err := s.permission.CanDownloadFile(ctx, actorID, fileID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}

	file, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if file.DeletedAt != nil {
		return nil, repository.ErrNotFound
	}
	return toDTO(file), nil
}

func (s *Service) Download(ctx context.Context, actorID int64, fileID int64) (*Download, error) {
	allowed, err := s.permission.CanDownloadFile(ctx, actorID, fileID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}

	file, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if file.DeletedAt != nil {
		return nil, repository.ErrNotFound
	}

	reader, err := s.fs.Open(ctx, file.RelativePath)
	if err != nil {
		return nil, err
	}

	return &Download{File: file, Reader: reader}, nil
}

func (s *Service) Delete(ctx context.Context, actorID int64, fileID int64) error {
	allowed, err := s.permission.CanDeleteFile(ctx, actorID, fileID)
	if err != nil {
		return err
	}
	if !allowed {
		return repository.ErrForbidden
	}

	return s.deleteContent(ctx, fileID, actorID, nil)
}

func (s *Service) deleteContent(ctx context.Context, fileID int64, actorID int64, beforeCommit func(context.Context) error) error {
	var deleted *domain.File
	return s.uow.WithinTx(ctx, func(ctx context.Context) error {
		if beforeCommit != nil {
			if err := beforeCommit(ctx); err != nil {
				return err
			}
		}

		file, err := s.files.SoftDelete(ctx, SoftDeleteFileParams{FileID: fileID, ActorID: actorID})
		if err != nil {
			return err
		}
		deleted = file

		_, err = s.storages.AdjustUsedSize(ctx, storagepkg.AdjustStorageSizeParams{
			StorageID: deleted.StorageID,
			Delta:     -deleted.Size,
		})
		return err
	})
}

func (s *Service) stageValidated(ctx context.Context, storageID int64, originalName string, reader io.Reader) (*filesystem.StagedFile, string, string, *domain.Storage, error) {
	storage, err := s.storages.GetByID(ctx, storageID)
	if err != nil {
		return nil, "", "", nil, err
	}
	if storage.DeletedAt != nil {
		return nil, "", "", nil, repository.ErrNotFound
	}

	name := cleanOriginalName(originalName)
	mimeType, fullReader, err := sniffMimeType(reader)
	if err != nil {
		return nil, "", "", nil, err
	}

	typeRules, err := s.storages.ListTypeRules(ctx, storageID)
	if err != nil {
		return nil, "", "", nil, err
	}
	if !fileTypeAllowed(name, mimeType, typeRules) {
		return nil, "", "", nil, repository.InvalidInput("Тип файла запрещен настройками хранилища")
	}

	storedName := uuid.NewString() + strings.ToLower(filepath.Ext(name))
	limitedReader := io.LimitReader(fullReader, storage.MaxFileSize+1)
	staged, err := s.fs.Stage(ctx, filesystem.SaveFileParams{
		Reader:      limitedReader,
		StoredName:  storedName,
		ContentType: mimeType,
	})
	if err != nil {
		return nil, "", "", nil, err
	}
	if staged.Size > storage.MaxFileSize {
		_ = s.fs.DeleteStaged(context.Background(), staged)
		return nil, "", "", nil, repository.LimitExceeded("Файл больше максимального размера для этого хранилища")
	}

	return staged, mimeType, name, storage, nil
}

func (s *Service) validateFolder(ctx context.Context, storageID int64, folderID *int64) error {
	if folderID == nil {
		return nil
	}
	if *folderID <= 0 {
		return repository.InvalidInput("Некорректная папка")
	}
	folder, err := s.folders.GetByID(ctx, *folderID)
	if err != nil {
		return err
	}
	if folder.DeletedAt != nil || folder.StorageID != storageID {
		return repository.ErrNotFound
	}
	return nil
}

func toDTO(file *domain.File) *FileDTO {
	return &FileDTO{
		ID:           file.ID,
		StorageID:    file.StorageID,
		FolderID:     file.FolderID,
		OwnerID:      file.OwnerID,
		OriginalName: file.OriginalName,
		MimeType:     file.MimeType,
		Size:         file.Size,
		Checksum:     file.Checksum,
	}
}

func sniffMimeType(reader io.Reader) (string, io.Reader, error) {
	buffer := make([]byte, 512)
	n, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		return "", nil, err
	}
	buffer = buffer[:n]

	mimeType := http.DetectContentType(buffer)
	mimeType = normalizeMime(mimeType)
	return mimeType, io.MultiReader(bytes.NewReader(buffer), reader), nil
}

func cleanOriginalName(originalName string) string {
	originalName = strings.ReplaceAll(originalName, "\\", "/")
	name := strings.TrimSpace(path.Base(originalName))
	if name == "" || name == "." || name == "/" {
		return "file"
	}
	return name
}

func normalizeMime(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	mediaType, _, err := mime.ParseMediaType(value)
	if err == nil {
		return mediaType
	}
	return value
}

func fileTypeAllowed(originalName string, actualMime string, rules []domain.StorageTypeRule) bool {
	actualMime = normalizeMime(actualMime)
	actualExt := strings.ToLower(filepath.Ext(originalName))

	hasAllowRules := false
	allowed := false
	for _, rule := range rules {
		switch rule.RuleType {
		case domain.StorageTypeRuleDeny:
			if fileTypeRuleMatches(actualExt, actualMime, rule.Pattern) {
				return false
			}
		case domain.StorageTypeRuleAllow:
			hasAllowRules = true
			if fileTypeRuleMatches(actualExt, actualMime, rule.Pattern) {
				allowed = true
			}
		}
	}
	return !hasAllowRules || allowed
}

func fileTypeRuleMatches(actualExt string, actualMime string, pattern string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	if pattern == "" {
		return false
	}
	if strings.Contains(pattern, "/") {
		return actualMime == normalizeMime(pattern)
	}
	return actualExt != "" && actualExt == normalizeExtensionRule(pattern)
}

func normalizeExtensionRule(value string) string {
	value = strings.TrimPrefix(strings.TrimSpace(value), "*")
	if value == "" {
		return ""
	}
	ext := filepath.Ext(value)
	if ext != "" {
		return strings.ToLower(ext)
	}
	if strings.HasPrefix(value, ".") {
		return strings.ToLower(value)
	}
	return "." + strings.ToLower(value)
}

func validateRenamedFileName(currentName string, nextName string) (string, error) {
	nextName = cleanOriginalName(nextName)
	if nextName == "" || nextName == "file" {
		return "", repository.InvalidInput("Название файла обязательно")
	}

	if strings.ToLower(filepath.Ext(currentName)) != strings.ToLower(filepath.Ext(nextName)) {
		return "", repository.InvalidInput("Расширение файла менять нельзя")
	}
	return nextName, nil
}
