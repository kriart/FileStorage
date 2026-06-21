package access

import (
	"context"
	"errors"
	"testing"
	"time"

	"file-storage-server/internal/domain"
	"file-storage-server/internal/folder"
	"file-storage-server/internal/repository"
	"file-storage-server/internal/user"
)

func TestStorageAccessServiceBlocksRemovingLastOwner(t *testing.T) {
	ctx := context.Background()
	accesses := newFakeAccessRepository([]domain.StorageAccess{
		{ID: 1, StorageID: 10, UserID: 1, AccessLevel: domain.StorageAccessOwner},
	})
	service := NewStorageAccessService(accesses, fakeUserRepository{}, fakeFolderRepository{}, allowGrantPermission{})

	err := service.Delete(ctx, 1, 10, 1)
	if !errors.Is(err, repository.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestStorageAccessServiceBlocksDemotingLastOwner(t *testing.T) {
	ctx := context.Background()
	accesses := newFakeAccessRepository([]domain.StorageAccess{
		{ID: 1, StorageID: 10, UserID: 1, AccessLevel: domain.StorageAccessOwner},
	})
	service := NewStorageAccessService(accesses, fakeUserRepository{}, fakeFolderRepository{}, allowGrantPermission{})

	_, err := service.Update(ctx, 1, 10, 1, UpdateStorageAccessDTO{AccessLevel: domain.StorageAccessManager})
	if !errors.Is(err, repository.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestStorageAccessServiceAllowsRemovingOwnerWhenAnotherOwnerExists(t *testing.T) {
	ctx := context.Background()
	accesses := newFakeAccessRepository([]domain.StorageAccess{
		{ID: 1, StorageID: 10, UserID: 1, AccessLevel: domain.StorageAccessOwner},
		{ID: 2, StorageID: 10, UserID: 2, AccessLevel: domain.StorageAccessOwner},
	})
	service := NewStorageAccessService(accesses, fakeUserRepository{}, fakeFolderRepository{}, allowGrantPermission{})

	err := service.Delete(ctx, 1, 10, 1)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if _, err := accesses.GetStorageAccess(ctx, 10, 1); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected deleted access to be absent, got %v", err)
	}
}

type allowGrantPermission struct{}

func (allowGrantPermission) CanGrantStorageAccess(context.Context, int64, int64) (bool, error) {
	return true, nil
}

type fakeUserRepository struct{}

func (fakeUserRepository) Create(context.Context, user.CreateUserParams) (*domain.User, error) {
	return nil, nil
}

func (fakeUserRepository) GetByID(_ context.Context, id int64) (*domain.User, error) {
	if id <= 0 {
		return nil, repository.ErrNotFound
	}
	return &domain.User{ID: id, Role: domain.UserRoleUser}, nil
}

func (fakeUserRepository) GetByEmail(context.Context, string) (*domain.User, error) {
	return nil, repository.ErrNotFound
}

func (fakeUserRepository) GetByUsername(context.Context, string) (*domain.User, error) {
	return nil, repository.ErrNotFound
}

func (fakeUserRepository) List(context.Context) ([]domain.User, error) {
	return nil, nil
}

func (fakeUserRepository) UpdateRole(context.Context, user.UpdateUserRoleParams) (*domain.User, error) {
	return nil, nil
}

type fakeAccessRepository struct {
	accesses       map[[2]int64]domain.StorageAccess
	folderAccesses map[[2]int64]domain.FolderAccess
	nextID         int64
}

func newFakeAccessRepository(accesses []domain.StorageAccess) *fakeAccessRepository {
	result := &fakeAccessRepository{
		accesses:       make(map[[2]int64]domain.StorageAccess, len(accesses)),
		folderAccesses: make(map[[2]int64]domain.FolderAccess),
		nextID:         1,
	}
	for _, access := range accesses {
		result.accesses[[2]int64{access.StorageID, access.UserID}] = access
		if access.ID >= result.nextID {
			result.nextID = access.ID + 1
		}
	}
	return result
}

func (r *fakeAccessRepository) UpsertFolderAccess(_ context.Context, params UpsertFolderAccessParams) (*domain.FolderAccess, error) {
	key := [2]int64{params.FolderID, params.UserID}
	access := domain.FolderAccess{
		ID:          r.nextID,
		FolderID:    params.FolderID,
		UserID:      params.UserID,
		AccessLevel: params.AccessLevel,
		CreatedAt:   time.Now(),
	}
	r.nextID++
	r.folderAccesses[key] = access
	return &access, nil
}

func (r *fakeAccessRepository) GetFolderAccess(_ context.Context, folderID int64, userID int64) (*domain.FolderAccess, error) {
	access, ok := r.folderAccesses[[2]int64{folderID, userID}]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &access, nil
}

func (r *fakeAccessRepository) ListFolderAccesses(_ context.Context, folderID int64) ([]domain.FolderAccess, error) {
	result := make([]domain.FolderAccess, 0)
	for _, access := range r.folderAccesses {
		if access.FolderID == folderID {
			result = append(result, access)
		}
	}
	return result, nil
}

func (r *fakeAccessRepository) DeleteFolderAccess(_ context.Context, folderID int64, userID int64) error {
	key := [2]int64{folderID, userID}
	if _, ok := r.folderAccesses[key]; !ok {
		return repository.ErrNotFound
	}
	delete(r.folderAccesses, key)
	return nil
}

type fakeFolderRepository struct{}

func (fakeFolderRepository) Create(context.Context, folder.CreateFolderParams) (*domain.Folder, error) {
	return nil, nil
}

func (fakeFolderRepository) GetByID(_ context.Context, id int64) (*domain.Folder, error) {
	if id <= 0 {
		return nil, repository.ErrNotFound
	}
	return &domain.Folder{ID: id, StorageID: 10}, nil
}

func (fakeFolderRepository) ListByStorage(context.Context, folder.ListFoldersFilter) ([]domain.Folder, error) {
	return nil, nil
}

func (fakeFolderRepository) Rename(context.Context, folder.RenameFolderParams) (*domain.Folder, error) {
	return nil, nil
}

func (fakeFolderRepository) SoftDeleteTree(context.Context, int64) (*folder.DeleteFolderTreeResult, error) {
	return nil, nil
}

func (r *fakeAccessRepository) UpsertStorageAccess(_ context.Context, params UpsertStorageAccessParams) (*domain.StorageAccess, error) {
	key := [2]int64{params.StorageID, params.UserID}
	access, ok := r.accesses[key]
	if !ok {
		access = domain.StorageAccess{
			ID:        r.nextID,
			StorageID: params.StorageID,
			UserID:    params.UserID,
			CreatedAt: time.Now(),
		}
		r.nextID++
	}

	access.AccessLevel = params.AccessLevel
	r.accesses[key] = access
	return &access, nil
}

func (r *fakeAccessRepository) GetStorageAccess(_ context.Context, storageID int64, userID int64) (*domain.StorageAccess, error) {
	access, ok := r.accesses[[2]int64{storageID, userID}]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &access, nil
}

func (r *fakeAccessRepository) ListStorageAccesses(_ context.Context, storageID int64) ([]domain.StorageAccess, error) {
	result := make([]domain.StorageAccess, 0)
	for _, access := range r.accesses {
		if access.StorageID == storageID {
			result = append(result, access)
		}
	}
	return result, nil
}

func (r *fakeAccessRepository) DeleteStorageAccess(_ context.Context, storageID int64, userID int64) error {
	key := [2]int64{storageID, userID}
	if _, ok := r.accesses[key]; !ok {
		return repository.ErrNotFound
	}
	delete(r.accesses, key)
	return nil
}
