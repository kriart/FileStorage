package access

import (
	"context"
	"errors"

	"file-storage-server/internal/domain"
	"file-storage-server/internal/file"
	"file-storage-server/internal/repository"
	"file-storage-server/internal/storage"
	"file-storage-server/internal/user"
)

type PermissionService struct {
	users    user.Repository
	storages storage.Repository
	files    file.Repository
	accesses Repository
}

func NewPermissionService(
	users user.Repository,
	storages storage.Repository,
	files file.Repository,
	accesses Repository,
) *PermissionService {
	return &PermissionService{
		users:    users,
		storages: storages,
		files:    files,
		accesses: accesses,
	}
}

func (s *PermissionService) CanViewStorage(ctx context.Context, userID int64, storageID int64) (bool, error) {
	currentUser, storage, level, err := s.userStorageAccess(ctx, userID, storageID)
	if err != nil {
		return false, err
	}
	if currentUser.Role == domain.UserRoleAdmin {
		return true, nil
	}
	if storage.Visibility == domain.StorageVisibilityPublicRead || storage.Visibility == domain.StorageVisibilityPublicUpload {
		return true, nil
	}
	return level != nil, nil
}

func (s *PermissionService) CanUploadToStorage(ctx context.Context, userID int64, storageID int64) (bool, error) {
	currentUser, storage, level, err := s.userStorageAccess(ctx, userID, storageID)
	if err != nil {
		return false, err
	}
	if currentUser.Role == domain.UserRoleAdmin {
		return true, nil
	}
	if storage.Visibility == domain.StorageVisibilityPublicUpload {
		return true, nil
	}
	return hasAtLeast(level, domain.StorageAccessUploader), nil
}

func (s *PermissionService) CanDownloadFile(ctx context.Context, userID int64, fileID int64) (bool, error) {
	file, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return false, err
	}
	if file.DeletedAt != nil {
		return false, nil
	}

	currentUser, storage, level, err := s.userStorageAccess(ctx, userID, file.StorageID)
	if err != nil {
		return false, err
	}
	if currentUser.Role == domain.UserRoleAdmin {
		return true, nil
	}
	if storage.Visibility == domain.StorageVisibilityPublicRead || storage.Visibility == domain.StorageVisibilityPublicUpload || level != nil {
		return true, nil
	}
	return s.hasFolderAccess(ctx, userID, file.FolderID, domain.StorageAccessViewer)
}

func (s *PermissionService) CanDeleteFile(ctx context.Context, userID int64, fileID int64) (bool, error) {
	file, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return false, err
	}
	if file.DeletedAt != nil {
		return false, nil
	}

	currentUser, _, level, err := s.userStorageAccess(ctx, userID, file.StorageID)
	if err != nil {
		return false, err
	}
	if currentUser.Role == domain.UserRoleAdmin {
		return true, nil
	}
	if hasAtLeast(level, domain.StorageAccessManager) {
		return true, nil
	}
	return s.hasFolderAccess(ctx, userID, file.FolderID, domain.StorageAccessManager)
}

func (s *PermissionService) CanEditFile(ctx context.Context, userID int64, fileID int64) (bool, error) {
	file, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return false, err
	}
	if file.DeletedAt != nil {
		return false, nil
	}

	currentUser, _, level, err := s.userStorageAccess(ctx, userID, file.StorageID)
	if err != nil {
		return false, err
	}
	if currentUser.Role == domain.UserRoleAdmin {
		return true, nil
	}
	if hasAtLeast(level, domain.StorageAccessUploader) {
		return true, nil
	}
	return s.hasFolderAccess(ctx, userID, file.FolderID, domain.StorageAccessUploader)
}

func (s *PermissionService) CanManageStorage(ctx context.Context, userID int64, storageID int64) (bool, error) {
	currentUser, _, level, err := s.userStorageAccess(ctx, userID, storageID)
	if err != nil {
		return false, err
	}
	if currentUser.Role == domain.UserRoleAdmin {
		return true, nil
	}
	return hasAtLeast(level, domain.StorageAccessManager), nil
}

func (s *PermissionService) CanGrantStorageAccess(ctx context.Context, userID int64, storageID int64) (bool, error) {
	currentUser, _, level, err := s.userStorageAccess(ctx, userID, storageID)
	if err != nil {
		return false, err
	}
	if currentUser.Role == domain.UserRoleAdmin {
		return true, nil
	}
	return hasAtLeast(level, domain.StorageAccessOwner), nil
}

func (s *PermissionService) CanShareFile(ctx context.Context, userID int64, fileID int64) (bool, error) {
	file, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return false, err
	}
	if file.DeletedAt != nil {
		return false, nil
	}

	currentUser, _, level, err := s.userStorageAccess(ctx, userID, file.StorageID)
	if err != nil {
		return false, err
	}
	if currentUser.Role == domain.UserRoleAdmin {
		return true, nil
	}
	return hasAtLeast(level, domain.StorageAccessManager), nil
}

func (s *PermissionService) userStorageAccess(
	ctx context.Context,
	userID int64,
	storageID int64,
) (*domain.User, *domain.Storage, *domain.StorageAccessLevel, error) {
	currentUser, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, nil, err
	}

	storage, err := s.storages.GetByID(ctx, storageID)
	if err != nil {
		return nil, nil, nil, err
	}
	if storage.DeletedAt != nil {
		return nil, nil, nil, repository.ErrNotFound
	}

	storageAccess, err := s.accesses.GetStorageAccess(ctx, storageID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return currentUser, storage, nil, nil
		}
		return nil, nil, nil, err
	}

	level := storageAccess.AccessLevel
	return currentUser, storage, &level, nil
}

func hasAtLeast(actual *domain.StorageAccessLevel, required domain.StorageAccessLevel) bool {
	if actual == nil {
		return false
	}
	return storageAccessRank(*actual) >= storageAccessRank(required)
}

func storageAccessRank(level domain.StorageAccessLevel) int {
	switch level {
	case domain.StorageAccessViewer:
		return 1
	case domain.StorageAccessUploader:
		return 2
	case domain.StorageAccessManager:
		return 3
	case domain.StorageAccessOwner:
		return 4
	default:
		return 0
	}
}

func (s *PermissionService) hasFolderAccess(ctx context.Context, userID int64, folderID *int64, required domain.StorageAccessLevel) (bool, error) {
	if folderID == nil {
		return false, nil
	}
	access, err := s.accesses.GetFolderAccess(ctx, *folderID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return hasAtLeast(&access.AccessLevel, required), nil
}
