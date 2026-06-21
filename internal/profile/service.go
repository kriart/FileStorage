package profile

import "context"

type Service struct {
	repository Store
}

func NewService(repository Store) *Service {
	return &Service{repository: repository}
}

func (s *Service) Get(ctx context.Context, userID int64) (*Profile, error) {
	return s.repository.Get(ctx, userID)
}

func (s *Service) UpdateSettings(ctx context.Context, userID int64, settings UpdateSettings) (*UserInfo, error) {
	return s.repository.UpdateSettings(ctx, userID, settings)
}

func (s *Service) UpdatePreferences(ctx context.Context, userID int64, preferences UpdatePreferences) (*UserInfo, error) {
	return s.repository.UpdatePreferences(ctx, userID, preferences)
}

func (s *Service) UpdateAvatarPath(ctx context.Context, userID int64, avatarPath string) (*UserInfo, error) {
	return s.repository.UpdateAvatarPath(ctx, userID, avatarPath)
}

func (s *Service) RevokeSession(ctx context.Context, userID int64, sessionID int64) error {
	return s.repository.RevokeSession(ctx, userID, sessionID)
}
