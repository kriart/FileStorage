package share

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"strings"

	"file-storage-server/internal/domain"
	"file-storage-server/internal/file"
	"file-storage-server/internal/filesystem"
	"file-storage-server/internal/repository"
)

type PermissionChecker interface {
	CanShareFile(ctx context.Context, userID int64, fileID int64) (bool, error)
}

type Service struct {
	links      Repository
	files      file.Repository
	fileSvc    FileReplaceService
	fs         filesystem.Storage
	permission PermissionChecker
}

type FileReplaceService interface {
	ReplaceFromShareWithHook(
		ctx context.Context,
		fileID int64,
		originalName string,
		reader io.Reader,
		beforeCommit func(context.Context) error,
	) (*file.FileDTO, error)
	RenameFromShareWithHook(ctx context.Context, fileID int64, name string, beforeCommit func(context.Context) error) (*file.FileDTO, error)
	DeleteFromShareWithHook(ctx context.Context, fileID int64, beforeCommit func(context.Context) error) error
}

type PublicDownload struct {
	File   *domain.File
	Reader io.ReadCloser
}

type PublicLinkInfo struct {
	Link *domain.ShareLink
	File *domain.File
}

func NewService(links Repository, files file.Repository, fileSvc FileReplaceService, fs filesystem.Storage, permission PermissionChecker) *Service {
	return &Service{
		links:      links,
		files:      files,
		fileSvc:    fileSvc,
		fs:         fs,
		permission: permission,
	}
}

func (s *Service) Create(ctx context.Context, actorID int64, fileID int64, dto CreateShareLinkDTO, baseURL string) (*ShareLinkDTO, error) {
	allowed, err := s.permission.CanShareFile(ctx, actorID, fileID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}
	if !validShareAccessType(dto.AccessType) {
		return nil, repository.ErrInvalidInput
	}

	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	link, err := s.links.Create(ctx, CreateShareLinkParams{
		FileID:     fileID,
		Token:      token,
		TokenHash:  hashToken(token),
		AccessType: dto.AccessType,
		ExpiresAt:  dto.ExpiresAt,
		CreatedBy:  actorID,
	})
	if err != nil {
		return nil, err
	}

	return toDTO(link, baseURL), nil
}

func (s *Service) List(ctx context.Context, actorID int64, fileID int64, baseURL string) ([]ShareLinkDTO, error) {
	allowed, err := s.permission.CanShareFile(ctx, actorID, fileID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, repository.ErrForbidden
	}

	links, err := s.links.ListByFile(ctx, ListShareLinksFilter{FileID: fileID})
	if err != nil {
		return nil, err
	}

	result := make([]ShareLinkDTO, 0, len(links))
	for i := range links {
		link := links[i]
		if link.Token == nil && link.IsActive {
			token, err := generateToken()
			if err != nil {
				return nil, err
			}
			updated, err := s.links.UpdateToken(ctx, link.ID, token, hashToken(token))
			if err != nil {
				return nil, err
			}
			link = *updated
		}
		result = append(result, *toDTO(&link, baseURL))
	}
	return result, nil
}

func (s *Service) Deactivate(ctx context.Context, actorID int64, linkID int64) error {
	link, err := s.links.GetByID(ctx, linkID)
	if err != nil {
		return err
	}

	allowed, err := s.permission.CanShareFile(ctx, actorID, link.FileID)
	if err != nil {
		return err
	}
	if !allowed {
		return repository.ErrForbidden
	}

	return s.links.Deactivate(ctx, linkID)
}

func (s *Service) PublicDownload(ctx context.Context, token string) (*PublicDownload, error) {
	info, err := s.PublicInfo(ctx, token)
	if err != nil {
		return nil, err
	}
	if info.Link.AccessType != domain.ShareAccessRead && info.Link.AccessType != domain.ShareAccessWrite {
		return nil, repository.ErrForbidden
	}

	if err := s.links.IncrementUseCount(ctx, info.Link.ID); err != nil {
		return nil, err
	}

	reader, err := s.fs.Open(ctx, info.File.RelativePath)
	if err != nil {
		return nil, err
	}

	return &PublicDownload{File: info.File, Reader: reader}, nil
}

func (s *Service) PublicReplace(ctx context.Context, token string, originalName string, reader io.Reader) (*file.FileDTO, error) {
	info, err := s.PublicInfo(ctx, token)
	if err != nil {
		return nil, err
	}
	if info.Link.AccessType != domain.ShareAccessWrite {
		return nil, repository.ErrForbidden
	}

	if info.File.DeletedAt != nil {
		return nil, repository.ErrNotFound
	}

	return s.fileSvc.ReplaceFromShareWithHook(ctx, info.File.ID, originalName, reader, func(ctx context.Context) error {
		return s.links.IncrementUseCount(ctx, info.Link.ID)
	})
}

func (s *Service) PublicRename(ctx context.Context, token string, name string) (*file.FileDTO, error) {
	info, err := s.PublicInfo(ctx, token)
	if err != nil {
		return nil, err
	}
	if info.Link.AccessType != domain.ShareAccessWrite {
		return nil, repository.ErrForbidden
	}

	return s.fileSvc.RenameFromShareWithHook(ctx, info.File.ID, name, func(ctx context.Context) error {
		return s.links.IncrementUseCount(ctx, info.Link.ID)
	})
}

func (s *Service) PublicDelete(ctx context.Context, token string) error {
	info, err := s.PublicInfo(ctx, token)
	if err != nil {
		return err
	}
	if info.Link.AccessType != domain.ShareAccessWrite {
		return repository.ErrForbidden
	}

	return s.fileSvc.DeleteFromShareWithHook(ctx, info.File.ID, func(ctx context.Context) error {
		return s.links.IncrementUseCount(ctx, info.Link.ID)
	})
}

func (s *Service) PublicInfo(ctx context.Context, token string) (*PublicLinkInfo, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, repository.ErrNotFound
	}

	link, err := s.links.GetByTokenHash(ctx, hashToken(token))
	if err != nil {
		return nil, err
	}

	fileModel, err := s.files.GetByID(ctx, link.FileID)
	if err != nil {
		return nil, err
	}
	if fileModel.DeletedAt != nil {
		return nil, repository.ErrNotFound
	}

	return &PublicLinkInfo{Link: link, File: fileModel}, nil
}

func toDTO(link *domain.ShareLink, baseURL string) *ShareLinkDTO {
	dto := &ShareLinkDTO{
		ID:         link.ID,
		FileID:     link.FileID,
		AccessType: link.AccessType,
		ExpiresAt:  link.ExpiresAt,
		UseCount:   link.UseCount,
		IsActive:   link.IsActive,
	}
	if link.Token != nil && *link.Token != "" && baseURL != "" {
		dto.URL = strings.TrimRight(baseURL, "/") + "/api/public/share/" + *link.Token
	}
	return dto
}

func validShareAccessType(accessType domain.ShareAccessType) bool {
	return accessType == domain.ShareAccessRead || accessType == domain.ShareAccessWrite
}

func generateToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
