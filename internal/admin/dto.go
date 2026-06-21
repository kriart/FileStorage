package admin

import "time"

type Dashboard struct {
	Stats    Stats
	Users    []UserRow
	Storages []StorageRow
	Audit    []AuditRow
}

type Stats struct {
	Users       int
	Storages    int
	Files       int
	ActiveLinks int
	UsedBytes   int64
}

type UserRow struct {
	ID        int64
	Email     string
	Username  string
	Role      string
	CreatedAt time.Time
}

type StorageRow struct {
	ID        int64
	Name      string
	Type      string
	UsedSize  int64
	Deleted   bool
	CreatedAt time.Time
}

type AuditRow struct {
	ID         int64
	UserEmail  *string
	Action     string
	EntityType string
	EntityID   *int64
	IP         string
	CreatedAt  time.Time
}
