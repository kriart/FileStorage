package storage

import (
	"context"
	"path/filepath"
	"strings"

	"file-storage-server/internal/domain"
	"file-storage-server/internal/repository"
)

const (
	DefaultMaxFileSize    int64 = 10 << 20
	DefaultMaxStorageSize int64 = 1 << 30
)

type PermissionChecker interface {
	CanViewStorage(ctx context.Context, userID int64, storageID int64) (bool, error)
	CanManageStorage(ctx context.Context, userID int64, storageID int64) (bool, error)
}

type Service struct {
	storages   Repository
	permission PermissionChecker
}

func NewService(storages Repository, permission PermissionChecker) *Service {
	return &Service{
		storages:   storages,
		permission: permission,
	}
}

func (s *Service) Create(ctx context.Context, actorID int64, actorRole domain.UserRole, dto CreateStorageDTO) (*StorageDTO, error) {
	params, err := normalizeCreateStorage(actorID, dto)
	if err != nil {
		return nil, err
	}
	if params.Type == domain.StorageTypeGlobal && actorRole != domain.UserRoleAdmin {
		return nil, repository.ErrForbidden
	}

	created, err := s.storages.Create(ctx, params)
	if err != nil {
		return nil, err
	}

	return s.toDTO(ctx, created)
}

func (s *Service) Get(ctx context.Context, actorID int64, storageID int64) (*StorageDTO, error) {
	allowed, err := s.permission.CanViewStorage(ctx, actorID, storageID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}

	storage, err := s.storages.GetByID(ctx, storageID)
	if err != nil {
		return nil, err
	}
	if storage.DeletedAt != nil {
		return nil, repository.ErrNotFound
	}
	return s.toDTO(ctx, storage)
}

func (s *Service) List(ctx context.Context, actorID int64) ([]StorageDTO, error) {
	storages, err := s.storages.ListAvailableForUser(ctx, ListStoragesFilter{UserID: actorID})
	if err != nil {
		return nil, err
	}

	result := make([]StorageDTO, 0, len(storages))
	for i := range storages {
		dto, err := s.toDTO(ctx, &storages[i])
		if err != nil {
			return nil, err
		}
		result = append(result, *dto)
	}
	return result, nil
}

func (s *Service) Update(ctx context.Context, actorID int64, storageID int64, dto UpdateStorageDTO) (*StorageDTO, error) {
	allowed, err := s.permission.CanManageStorage(ctx, actorID, storageID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}

	params, err := normalizeUpdateStorage(storageID, dto)
	if err != nil {
		return nil, err
	}

	updated, err := s.storages.Update(ctx, params)
	if err != nil {
		return nil, err
	}
	return s.toDTO(ctx, updated)
}

func (s *Service) Delete(ctx context.Context, actorID int64, storageID int64) error {
	allowed, err := s.permission.CanManageStorage(ctx, actorID, storageID)
	if err != nil {
		return err
	}
	if !allowed {
		return repository.ErrForbidden
	}
	return s.storages.SoftDelete(ctx, storageID)
}

func (s *Service) toDTO(ctx context.Context, storage *domain.Storage) (*StorageDTO, error) {
	rules, err := s.storages.ListTypeRules(ctx, storage.ID)
	if err != nil {
		return nil, err
	}
	allowedTypes, blockedTypes := splitRules(rules)

	return &StorageDTO{
		ID:               storage.ID,
		Name:             storage.Name,
		Type:             storage.Type,
		Visibility:       storage.Visibility,
		MaxFileSize:      storage.MaxFileSize,
		MaxStorageSize:   storage.MaxStorageSize,
		UsedSize:         storage.UsedSize,
		AllowedFileTypes: allowedTypes,
		BlockedFileTypes: blockedTypes,
	}, nil
}

func normalizeCreateStorage(actorID int64, dto CreateStorageDTO) (CreateStorageParams, error) {
	name := strings.TrimSpace(dto.Name)
	if name == "" {
		return CreateStorageParams{}, repository.InvalidInput("Название хранилища обязательно")
	}

	storageType := dto.Type
	if storageType == "" {
		storageType = domain.StorageTypePersonal
	}
	if storageType != domain.StorageTypePersonal && storageType != domain.StorageTypeGlobal {
		return CreateStorageParams{}, repository.InvalidInput("Некорректный тип хранилища")
	}

	visibility := dto.Visibility
	if visibility == "" {
		visibility = domain.StorageVisibilityPrivate
	}
	if !validVisibility(visibility) {
		return CreateStorageParams{}, repository.InvalidInput("Некорректная видимость хранилища")
	}

	maxFileSize := dto.MaxFileSize
	if maxFileSize == 0 {
		maxFileSize = DefaultMaxFileSize
	}
	maxStorageSize := dto.MaxStorageSize
	if maxStorageSize == 0 {
		maxStorageSize = DefaultMaxStorageSize
	}
	if maxFileSize <= 0 || maxStorageSize <= 0 || maxFileSize > maxStorageSize {
		return CreateStorageParams{}, repository.InvalidInput("Лимиты должны быть положительными, а максимальный размер файла не должен превышать размер хранилища")
	}

	return CreateStorageParams{
		Name:             name,
		Type:             storageType,
		Visibility:       visibility,
		MaxFileSize:      maxFileSize,
		MaxStorageSize:   maxStorageSize,
		AllowedFileTypes: normalizeTypeRules(dto.AllowedFileTypes),
		BlockedFileTypes: normalizeTypeRules(dto.BlockedFileTypes),
		OwnerID:          actorID,
	}, nil
}

func normalizeUpdateStorage(storageID int64, dto UpdateStorageDTO) (UpdateStorageParams, error) {
	params := UpdateStorageParams{
		StorageID:        storageID,
		Visibility:       dto.Visibility,
		MaxFileSize:      dto.MaxFileSize,
		MaxStorageSize:   dto.MaxStorageSize,
		AllowedFileTypes: dto.AllowedFileTypes,
		BlockedFileTypes: dto.BlockedFileTypes,
	}

	if dto.Name != nil {
		name := strings.TrimSpace(*dto.Name)
		if name == "" {
			return UpdateStorageParams{}, repository.InvalidInput("Название хранилища обязательно")
		}
		params.Name = &name
	}
	if dto.Visibility != nil && !validVisibility(*dto.Visibility) {
		return UpdateStorageParams{}, repository.InvalidInput("Некорректная видимость хранилища")
	}
	if dto.MaxFileSize != nil && *dto.MaxFileSize <= 0 {
		return UpdateStorageParams{}, repository.InvalidInput("Максимальный размер файла должен быть положительным")
	}
	if dto.MaxStorageSize != nil && *dto.MaxStorageSize <= 0 {
		return UpdateStorageParams{}, repository.InvalidInput("Максимальный размер хранилища должен быть положительным")
	}
	if dto.MaxFileSize != nil && dto.MaxStorageSize != nil && *dto.MaxFileSize > *dto.MaxStorageSize {
		return UpdateStorageParams{}, repository.InvalidInput("Максимальный размер файла не должен превышать размер хранилища")
	}
	if dto.AllowedFileTypes != nil {
		allowedTypes := normalizeTypeRules(*dto.AllowedFileTypes)
		params.AllowedFileTypes = &allowedTypes
	}
	if dto.BlockedFileTypes != nil {
		blockedTypes := normalizeTypeRules(*dto.BlockedFileTypes)
		params.BlockedFileTypes = &blockedTypes
	}

	return params, nil
}

func normalizeMimeTypes(mimeTypes []string) []string {
	return normalizeTypeRules(mimeTypes)
}

func normalizeTypeRules(mimeTypes []string) []string {
	result := make([]string, 0, len(mimeTypes))
	seen := make(map[string]struct{}, len(mimeTypes))

	for _, mimeType := range mimeTypes {
		rule := normalizeFileTypeRule(mimeType)
		if rule == "" {
			continue
		}
		if _, ok := seen[rule]; ok {
			continue
		}
		seen[rule] = struct{}{}
		result = append(result, rule)
	}

	return result
}

func splitRules(rules []domain.StorageTypeRule) ([]string, []string) {
	allowedTypes := make([]string, 0)
	blockedTypes := make([]string, 0)
	for _, rule := range rules {
		switch rule.RuleType {
		case domain.StorageTypeRuleAllow:
			allowedTypes = append(allowedTypes, rule.Pattern)
		case domain.StorageTypeRuleDeny:
			blockedTypes = append(blockedTypes, rule.Pattern)
		}
	}
	return allowedTypes, blockedTypes
}

func normalizeFileTypeRule(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "*")
	if value == "" {
		return ""
	}
	if strings.Contains(value, "/") {
		return value
	}

	ext := filepath.Ext(value)
	if ext != "" {
		return ext
	}
	if strings.HasPrefix(value, ".") {
		return value
	}
	return "." + value
}

func validVisibility(visibility domain.StorageVisibility) bool {
	return visibility == domain.StorageVisibilityPrivate ||
		visibility == domain.StorageVisibilityPublicRead ||
		visibility == domain.StorageVisibilityPublicUpload
}
