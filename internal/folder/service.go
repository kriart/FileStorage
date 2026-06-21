package folder

import (
	"context"
	"strings"

	"file-storage-server/internal/domain"
	"file-storage-server/internal/repository"
	storagepkg "file-storage-server/internal/storage"
)

type PermissionChecker interface {
	CanViewStorage(ctx context.Context, userID int64, storageID int64) (bool, error)
	CanUploadToStorage(ctx context.Context, userID int64, storageID int64) (bool, error)
	CanManageStorage(ctx context.Context, userID int64, storageID int64) (bool, error)
}

type Service struct {
	folders    Repository
	storages   storagepkg.Repository
	uow        repository.UnitOfWork
	permission PermissionChecker
}

func NewService(folders Repository, storages storagepkg.Repository, uow repository.UnitOfWork, permission PermissionChecker) *Service {
	return &Service{folders: folders, storages: storages, uow: uow, permission: permission}
}

func (s *Service) Create(ctx context.Context, actorID int64, dto CreateFolderDTO) (*FolderDTO, error) {
	name := cleanName(dto.Name)
	if dto.StorageID <= 0 || name == "" {
		return nil, repository.InvalidInput("Название папки обязательно")
	}

	allowed, err := s.permission.CanUploadToStorage(ctx, actorID, dto.StorageID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}

	if err := s.validateParent(ctx, dto.StorageID, dto.ParentID); err != nil {
		return nil, err
	}

	created, err := s.folders.Create(ctx, CreateFolderParams{
		StorageID: dto.StorageID,
		ParentID:  dto.ParentID,
		Name:      name,
		CreatedBy: actorID,
	})
	if err != nil {
		return nil, err
	}
	return toDTO(created), nil
}

func (s *Service) List(ctx context.Context, actorID int64, storageID int64, parentID *int64) ([]FolderDTO, error) {
	allowed, err := s.permission.CanViewStorage(ctx, actorID, storageID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}
	if err := s.validateParent(ctx, storageID, parentID); err != nil {
		return nil, err
	}

	folders, err := s.folders.ListByStorage(ctx, ListFoldersFilter{StorageID: storageID, ParentID: parentID})
	if err != nil {
		return nil, err
	}

	result := make([]FolderDTO, 0, len(folders))
	for i := range folders {
		result = append(result, *toDTO(&folders[i]))
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, actorID int64, folderID int64) (*FolderDTO, error) {
	folder, err := s.folders.GetByID(ctx, folderID)
	if err != nil {
		return nil, err
	}
	if folder.DeletedAt != nil {
		return nil, repository.ErrNotFound
	}

	allowed, err := s.permission.CanViewStorage(ctx, actorID, folder.StorageID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}
	return toDTO(folder), nil
}

func (s *Service) Rename(ctx context.Context, actorID int64, folderID int64, dto RenameFolderDTO) (*FolderDTO, error) {
	name := cleanName(dto.Name)
	if folderID <= 0 || name == "" {
		return nil, repository.InvalidInput("Название папки обязательно")
	}

	current, err := s.folders.GetByID(ctx, folderID)
	if err != nil {
		return nil, err
	}
	if current.DeletedAt != nil {
		return nil, repository.ErrNotFound
	}

	allowed, err := s.permission.CanManageStorage(ctx, actorID, current.StorageID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}

	renamed, err := s.folders.Rename(ctx, RenameFolderParams{FolderID: folderID, Name: name})
	if err != nil {
		return nil, err
	}
	return toDTO(renamed), nil
}

func (s *Service) Delete(ctx context.Context, actorID int64, folderID int64) (*FolderDTO, error) {
	if folderID <= 0 {
		return nil, repository.InvalidInput("Некорректная папка")
	}

	current, err := s.folders.GetByID(ctx, folderID)
	if err != nil {
		return nil, err
	}
	if current.DeletedAt != nil {
		return nil, repository.ErrNotFound
	}

	allowed, err := s.permission.CanManageStorage(ctx, actorID, current.StorageID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}

	var deleted *FolderDTO
	err = s.uow.WithinTx(ctx, func(ctx context.Context) error {
		result, err := s.folders.SoftDeleteTree(ctx, folderID)
		if err != nil {
			return err
		}
		if result.DeletedFileSize > 0 {
			if _, err := s.storages.AdjustUsedSize(ctx, storagepkg.AdjustStorageSizeParams{
				StorageID: result.Folder.StorageID,
				Delta:     -result.DeletedFileSize,
			}); err != nil {
				return err
			}
		}
		deleted = toDTO(&result.Folder)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return deleted, nil
}

func (s *Service) Path(ctx context.Context, actorID int64, storageID int64, folderID *int64) ([]FolderDTO, error) {
	if folderID == nil {
		return nil, nil
	}

	current, err := s.Get(ctx, actorID, *folderID)
	if err != nil {
		return nil, err
	}
	if current.StorageID != storageID {
		return nil, repository.ErrNotFound
	}

	path := []FolderDTO{*current}
	parentID := current.ParentID
	for parentID != nil {
		parent, err := s.Get(ctx, actorID, *parentID)
		if err != nil {
			return nil, err
		}
		if parent.StorageID != storageID {
			return nil, repository.ErrNotFound
		}
		path = append([]FolderDTO{*parent}, path...)
		parentID = parent.ParentID
	}
	return path, nil
}

func (s *Service) validateParent(ctx context.Context, storageID int64, parentID *int64) error {
	if parentID == nil {
		return nil
	}
	if *parentID <= 0 {
		return repository.InvalidInput("Некорректная родительская папка")
	}
	parent, err := s.folders.GetByID(ctx, *parentID)
	if err != nil {
		return err
	}
	if parent.DeletedAt != nil || parent.StorageID != storageID {
		return repository.ErrNotFound
	}
	return nil
}

func cleanName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	return name
}

func toDTO(folder *domain.Folder) *FolderDTO {
	return &FolderDTO{
		ID:        folder.ID,
		StorageID: folder.StorageID,
		ParentID:  folder.ParentID,
		Name:      folder.Name,
	}
}
