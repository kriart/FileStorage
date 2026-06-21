package profile

import "context"

type Store interface {
	Get(ctx context.Context, userID int64) (*Profile, error)
	UpdateSettings(ctx context.Context, userID int64, settings UpdateSettings) (*UserInfo, error)
	UpdatePreferences(ctx context.Context, userID int64, preferences UpdatePreferences) (*UserInfo, error)
	UpdateAvatarPath(ctx context.Context, userID int64, avatarPath string) (*UserInfo, error)
	RevokeSession(ctx context.Context, userID int64, sessionID int64) error
}
