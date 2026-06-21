package profile

import "time"

type Profile struct {
	User     UserInfo
	Sessions []Session
	Audit    []AuditEvent
}

type UserInfo struct {
	ID          int64
	Username    string
	Email       string
	Role        string
	AvatarPath  *string
	DateOfBirth *time.Time
	Theme       string
	Language    string
	CreatedAt   time.Time
}

type UpdateSettings struct {
	Username    string
	DateOfBirth *time.Time
}

type UpdatePreferences struct {
	Theme    string
	Language string
}

type Session struct {
	ID        int64
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
	Active    bool
}

type AuditEvent struct {
	ID         int64
	Action     string
	EntityType string
	EntityID   *int64
	IP         string
	CreatedAt  time.Time
}
