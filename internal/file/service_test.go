package file

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"file-storage-server/internal/domain"
	"file-storage-server/internal/filesystem"
	"file-storage-server/internal/folder"
	"file-storage-server/internal/repository"
	storagepkg "file-storage-server/internal/storage"
)

func TestFileTypeAllowedMatchesArbitraryExtension(t *testing.T) {
	if !fileTypeAllowed("archive.custom-format", "application/octet-stream", []domain.StorageTypeRule{
		{RuleType: domain.StorageTypeRuleAllow, Pattern: ".custom-format"},
	}) {
		t.Fatal("expected arbitrary extension to be allowed")
	}
}

func TestFileTypeAllowedMatchesMIME(t *testing.T) {
	if !fileTypeAllowed("picture.bin", "image/png; charset=binary", []domain.StorageTypeRule{
		{RuleType: domain.StorageTypeRuleAllow, Pattern: "image/png"},
	}) {
		t.Fatal("expected MIME rule to be allowed")
	}
}

func TestFileTypeAllowedRejectsDifferentExtensionAndMIME(t *testing.T) {
	if fileTypeAllowed("notes.exe", "application/octet-stream", []domain.StorageTypeRule{
		{RuleType: domain.StorageTypeRuleAllow, Pattern: ".txt"},
		{RuleType: domain.StorageTypeRuleAllow, Pattern: "application/pdf"},
	}) {
		t.Fatal("expected file to be rejected")
	}
}

func TestFileTypeAllowedDenyRulesWin(t *testing.T) {
	if fileTypeAllowed("setup.exe", "application/octet-stream", []domain.StorageTypeRule{
		{RuleType: domain.StorageTypeRuleDeny, Pattern: ".exe"},
	}) {
		t.Fatal("expected denied extension to be rejected")
	}
	if fileTypeAllowed("setup.exe", "application/octet-stream", []domain.StorageTypeRule{
		{RuleType: domain.StorageTypeRuleAllow, Pattern: ".exe"},
		{RuleType: domain.StorageTypeRuleDeny, Pattern: ".exe"},
	}) {
		t.Fatal("expected deny rule to override allow rule")
	}
}

func TestUploadCleansCommittedFileWhenDatabaseCreateFails(t *testing.T) {
	ctx := context.Background()
	createErr := errors.New("create failed")
	fs := &fakeStorageFS{}
	service := NewService(
		fakeFileRepository{createErr: createErr},
		fakeStorageRepository{storage: domain.Storage{ID: 10, MaxFileSize: 1024, MaxStorageSize: 4096}},
		fakeFolderRepository{},
		fs,
		fakeUnitOfWork{},
		fakePermissionChecker{canUpload: true},
	)

	_, err := service.Upload(ctx, 20, 10, nil, "notes.txt", strings.NewReader("hello"))
	if !errors.Is(err, createErr) {
		t.Fatalf("expected create error, got %v", err)
	}
	if fs.stageCalls != 1 {
		t.Fatalf("expected one staged file, got %d", fs.stageCalls)
	}
	if fs.commitCalls != 1 {
		t.Fatalf("expected one committed file, got %d", fs.commitCalls)
	}
	if !fs.deletedFinal["files/final.txt"] {
		t.Fatal("expected committed file to be cleaned up")
	}
}

func TestUploadReportsStorageCapacityLimit(t *testing.T) {
	ctx := context.Background()
	fs := &fakeStorageFS{}
	service := NewService(
		fakeFileRepository{},
		fakeStorageRepository{storage: domain.Storage{ID: 10, UsedSize: 4094, MaxFileSize: 1024, MaxStorageSize: 4096}},
		fakeFolderRepository{},
		fs,
		fakeUnitOfWork{},
		fakePermissionChecker{canUpload: true},
	)

	_, err := service.Upload(ctx, 20, 10, nil, "notes.txt", strings.NewReader("hello"))
	if !errors.Is(err, repository.ErrLimitExceeded) {
		t.Fatalf("expected limit error, got %v", err)
	}
	if got := repository.PublicMessage(err); got != "Файл не загружен: в хранилище недостаточно свободного места" {
		t.Fatalf("unexpected message: %q", got)
	}
	if !fs.deletedFinal["files/final.txt"] {
		t.Fatal("expected committed file to be cleaned up after capacity error")
	}
}

func TestValidateRenamedFileNameRejectsExtensionChange(t *testing.T) {
	_, err := validateRenamedFileName("report.docx", "report.pdf")
	if !errors.Is(err, repository.ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestValidateRenamedFileNameAllowsSameExtension(t *testing.T) {
	got, err := validateRenamedFileName("report.docx", "summary.DOCX")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != "summary.DOCX" {
		t.Fatalf("expected cleaned name, got %q", got)
	}
}

type fakeFileRepository struct {
	createErr error
}

func (r fakeFileRepository) Create(context.Context, CreateFileParams) (*domain.File, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	return &domain.File{ID: 1, StorageID: 10, OwnerID: 20, OriginalName: "notes.txt"}, nil
}

func (r fakeFileRepository) GetByID(context.Context, int64) (*domain.File, error) {
	return nil, repository.ErrNotFound
}

func (r fakeFileRepository) ListByStorage(context.Context, ListFilesFilter) ([]domain.File, error) {
	return nil, nil
}

func (r fakeFileRepository) CountByStorage(context.Context, ListFilesFilter) (int, error) {
	return 0, nil
}

func (r fakeFileRepository) UpdateContent(context.Context, UpdateFileContentParams) (*domain.File, error) {
	return nil, repository.ErrNotFound
}

func (r fakeFileRepository) Rename(context.Context, RenameFileParams) (*domain.File, error) {
	return nil, repository.ErrNotFound
}

func (r fakeFileRepository) SoftDelete(context.Context, SoftDeleteFileParams) (*domain.File, error) {
	return nil, repository.ErrNotFound
}

type fakeStorageRepository struct {
	storage domain.Storage
}

func (r fakeStorageRepository) Create(context.Context, storagepkg.CreateStorageParams) (*domain.Storage, error) {
	return nil, nil
}

func (r fakeStorageRepository) GetByID(context.Context, int64) (*domain.Storage, error) {
	return &r.storage, nil
}

func (r fakeStorageRepository) ListAvailableForUser(context.Context, storagepkg.ListStoragesFilter) ([]domain.Storage, error) {
	return nil, nil
}

func (r fakeStorageRepository) Update(context.Context, storagepkg.UpdateStorageParams) (*domain.Storage, error) {
	return nil, nil
}

func (r fakeStorageRepository) SoftDelete(context.Context, int64) error {
	return nil
}

func (r fakeStorageRepository) ListTypeRules(context.Context, int64) ([]domain.StorageTypeRule, error) {
	return nil, nil
}

func (r fakeStorageRepository) ReplaceTypeRules(context.Context, int64, []domain.StorageTypeRule) error {
	return nil
}

func (r fakeStorageRepository) AdjustUsedSize(context.Context, storagepkg.AdjustStorageSizeParams) (*domain.Storage, error) {
	return &r.storage, nil
}

type fakeFolderRepository struct{}

func (r fakeFolderRepository) Create(context.Context, folder.CreateFolderParams) (*domain.Folder, error) {
	return nil, nil
}

func (r fakeFolderRepository) GetByID(context.Context, int64) (*domain.Folder, error) {
	return nil, repository.ErrNotFound
}

func (r fakeFolderRepository) ListByStorage(context.Context, folder.ListFoldersFilter) ([]domain.Folder, error) {
	return nil, nil
}

func (r fakeFolderRepository) Rename(context.Context, folder.RenameFolderParams) (*domain.Folder, error) {
	return nil, nil
}

func (r fakeFolderRepository) SoftDeleteTree(context.Context, int64) (*folder.DeleteFolderTreeResult, error) {
	return nil, nil
}

type fakeStorageFS struct {
	stageCalls   int
	commitCalls  int
	deletedFinal map[string]bool
}

func (fs *fakeStorageFS) Save(ctx context.Context, params filesystem.SaveFileParams) (*filesystem.SavedFile, error) {
	staged, err := fs.Stage(ctx, params)
	if err != nil {
		return nil, err
	}
	return fs.Commit(ctx, staged)
}

func (fs *fakeStorageFS) Stage(context.Context, filesystem.SaveFileParams) (*filesystem.StagedFile, error) {
	fs.stageCalls++
	return &filesystem.StagedFile{
		StoredName:   "final.txt",
		RelativePath: "files/final.txt",
		TempPath:     "tmp/upload",
		Size:         5,
		Checksum:     "checksum",
	}, nil
}

func (fs *fakeStorageFS) Commit(context.Context, *filesystem.StagedFile) (*filesystem.SavedFile, error) {
	fs.commitCalls++
	return &filesystem.SavedFile{
		StoredName:   "final.txt",
		RelativePath: "files/final.txt",
		Size:         5,
		Checksum:     "checksum",
	}, nil
}

func (fs *fakeStorageFS) DeleteStaged(context.Context, *filesystem.StagedFile) error {
	return nil
}

func (fs *fakeStorageFS) Open(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (fs *fakeStorageFS) Delete(_ context.Context, relativePath string) error {
	if fs.deletedFinal == nil {
		fs.deletedFinal = make(map[string]bool)
	}
	fs.deletedFinal[relativePath] = true
	return nil
}

type fakeUnitOfWork struct{}

func (fakeUnitOfWork) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type fakePermissionChecker struct {
	canUpload bool
}

func (p fakePermissionChecker) CanViewStorage(context.Context, int64, int64) (bool, error) {
	return false, nil
}

func (p fakePermissionChecker) CanUploadToStorage(context.Context, int64, int64) (bool, error) {
	return p.canUpload, nil
}

func (p fakePermissionChecker) CanDownloadFile(context.Context, int64, int64) (bool, error) {
	return false, nil
}

func (p fakePermissionChecker) CanDeleteFile(context.Context, int64, int64) (bool, error) {
	return false, nil
}

func (p fakePermissionChecker) CanEditFile(context.Context, int64, int64) (bool, error) {
	return false, nil
}
