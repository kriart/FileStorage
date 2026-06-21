package access

import (
	"context"
	"errors"
	"strings"

	"file-storage-server/internal/domain"
	"file-storage-server/internal/folder"
	"file-storage-server/internal/repository"
	"file-storage-server/internal/user"
)

type StorageAccessService struct {
	accesses   Repository
	users      user.Repository
	folders    folder.Repository
	permission StorageAccessPermissionChecker
}

type StorageAccessPermissionChecker interface {
	CanGrantStorageAccess(ctx context.Context, userID int64, storageID int64) (bool, error)
}

func NewStorageAccessService(accesses Repository, users user.Repository, folders folder.Repository, permission StorageAccessPermissionChecker) *StorageAccessService {
	return &StorageAccessService{
		accesses:   accesses,
		users:      users,
		folders:    folders,
		permission: permission,
	}
}

func (s *StorageAccessService) List(ctx context.Context, actorID int64, storageID int64) ([]StorageAccessDTO, error) {
	allowed, err := s.permission.CanGrantStorageAccess(ctx, actorID, storageID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}

	accesses, err := s.accesses.ListStorageAccesses(ctx, storageID)
	if err != nil {
		return nil, err
	}

	result := make([]StorageAccessDTO, 0, len(accesses))
	for _, access := range accesses {
		accessUser, err := s.users.GetByID(ctx, access.UserID)
		if err != nil {
			return nil, err
		}
		result = append(result, StorageAccessDTO{
			UserID:      access.UserID,
			UserEmail:   accessUser.Email,
			StorageID:   access.StorageID,
			AccessLevel: access.AccessLevel,
		})
	}
	return result, nil
}

func (s *StorageAccessService) Grant(ctx context.Context, actorID int64, storageID int64, dto GrantStorageAccessDTO) (*StorageAccessDTO, error) {
	allowed, err := s.permission.CanGrantStorageAccess(ctx, actorID, storageID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}
	if !validAccessLevel(dto.AccessLevel) {
		return nil, repository.ErrInvalidInput
	}

	targetUser, err := s.resolveTargetUser(ctx, dto)
	if err != nil {
		return nil, err
	}
	if dto.AccessLevel != domain.StorageAccessOwner {
		if err := s.ensureCanRemoveOwner(ctx, storageID, targetUser.ID); err != nil {
			return nil, err
		}
	}

	access, err := s.accesses.UpsertStorageAccess(ctx, UpsertStorageAccessParams{
		StorageID:   storageID,
		UserID:      targetUser.ID,
		AccessLevel: dto.AccessLevel,
	})
	if err != nil {
		return nil, err
	}
	result := toStorageAccessDTO(access)
	result.UserEmail = targetUser.Email
	return result, nil
}

func (s *StorageAccessService) Update(ctx context.Context, actorID int64, storageID int64, userID int64, dto UpdateStorageAccessDTO) (*StorageAccessDTO, error) {
	return s.Grant(ctx, actorID, storageID, GrantStorageAccessDTO{
		UserID:      userID,
		AccessLevel: dto.AccessLevel,
	})
}

func (s *StorageAccessService) Delete(ctx context.Context, actorID int64, storageID int64, userID int64) error {
	allowed, err := s.permission.CanGrantStorageAccess(ctx, actorID, storageID)
	if err != nil {
		return err
	}
	if !allowed {
		return repository.ErrForbidden
	}
	if userID <= 0 {
		return repository.ErrInvalidInput
	}
	if err := s.ensureCanRemoveOwner(ctx, storageID, userID); err != nil {
		return err
	}

	return s.accesses.DeleteStorageAccess(ctx, storageID, userID)
}

func (s *StorageAccessService) ListFolder(ctx context.Context, actorID int64, folderID int64) ([]FolderAccessDTO, error) {
	folderModel, err := s.folderForManage(ctx, actorID, folderID)
	if err != nil {
		return nil, err
	}

	accesses, err := s.accesses.ListFolderAccesses(ctx, folderModel.ID)
	if err != nil {
		return nil, err
	}

	result := make([]FolderAccessDTO, 0, len(accesses))
	for _, access := range accesses {
		accessUser, err := s.users.GetByID(ctx, access.UserID)
		if err != nil {
			return nil, err
		}
		result = append(result, FolderAccessDTO{
			UserID:      access.UserID,
			UserEmail:   accessUser.Email,
			FolderID:    access.FolderID,
			AccessLevel: access.AccessLevel,
		})
	}
	return result, nil
}

func (s *StorageAccessService) GrantFolder(ctx context.Context, actorID int64, folderID int64, dto GrantStorageAccessDTO) (*FolderAccessDTO, error) {
	folderModel, err := s.folderForManage(ctx, actorID, folderID)
	if err != nil {
		return nil, err
	}
	if !validFolderAccessLevel(dto.AccessLevel) {
		return nil, repository.InvalidInput("Для папки доступны уровни: просмотр, загрузка или управление")
	}

	targetUser, err := s.resolveTargetUser(ctx, dto)
	if err != nil {
		return nil, err
	}

	access, err := s.accesses.UpsertFolderAccess(ctx, UpsertFolderAccessParams{
		FolderID:    folderModel.ID,
		UserID:      targetUser.ID,
		AccessLevel: dto.AccessLevel,
	})
	if err != nil {
		return nil, err
	}

	return &FolderAccessDTO{
		UserID:      targetUser.ID,
		UserEmail:   targetUser.Email,
		FolderID:    access.FolderID,
		AccessLevel: access.AccessLevel,
	}, nil
}

func (s *StorageAccessService) DeleteFolder(ctx context.Context, actorID int64, folderID int64, userID int64) error {
	if _, err := s.folderForManage(ctx, actorID, folderID); err != nil {
		return err
	}
	if userID <= 0 {
		return repository.ErrInvalidInput
	}
	return s.accesses.DeleteFolderAccess(ctx, folderID, userID)
}

func toStorageAccessDTO(access *domain.StorageAccess) *StorageAccessDTO {
	return &StorageAccessDTO{
		UserID:      access.UserID,
		StorageID:   access.StorageID,
		AccessLevel: access.AccessLevel,
	}
}

func (s *StorageAccessService) resolveTargetUser(ctx context.Context, dto GrantStorageAccessDTO) (*domain.User, error) {
	if dto.UserID > 0 {
		return s.users.GetByID(ctx, dto.UserID)
	}
	email := strings.TrimSpace(dto.UserEmail)
	if email == "" {
		return nil, repository.ErrInvalidInput
	}
	return s.users.GetByEmail(ctx, email)
}

func validAccessLevel(level domain.StorageAccessLevel) bool {
	return level == domain.StorageAccessOwner ||
		level == domain.StorageAccessManager ||
		level == domain.StorageAccessUploader ||
		level == domain.StorageAccessViewer
}

func validFolderAccessLevel(level domain.StorageAccessLevel) bool {
	return level == domain.StorageAccessManager ||
		level == domain.StorageAccessUploader ||
		level == domain.StorageAccessViewer
}

func (s *StorageAccessService) folderForManage(ctx context.Context, actorID int64, folderID int64) (*domain.Folder, error) {
	if folderID <= 0 {
		return nil, repository.ErrInvalidInput
	}
	folderModel, err := s.folders.GetByID(ctx, folderID)
	if err != nil {
		return nil, err
	}
	if folderModel.DeletedAt != nil {
		return nil, repository.ErrNotFound
	}

	allowed, err := s.permission.CanGrantStorageAccess(ctx, actorID, folderModel.StorageID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}
	return folderModel, nil
}

func (s *StorageAccessService) ensureCanRemoveOwner(ctx context.Context, storageID int64, userID int64) error {
	currentAccess, err := s.accesses.GetStorageAccess(ctx, storageID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return err
	}
	if currentAccess.AccessLevel != domain.StorageAccessOwner {
		return nil
	}

	accesses, err := s.accesses.ListStorageAccesses(ctx, storageID)
	if err != nil {
		return err
	}

	ownerCount := 0
	for _, access := range accesses {
		if access.AccessLevel == domain.StorageAccessOwner {
			ownerCount++
		}
	}
	if ownerCount <= 1 {
		return repository.ErrInvalidInput
	}
	return nil
}
