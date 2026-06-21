package api

import (
	"net/http"
	"strconv"
	"strings"

	"file-storage-server/internal/audit"
	"file-storage-server/internal/auth"
	"file-storage-server/internal/repository"
)

func parseID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, repository.ErrInvalidInput
	}
	return id, nil
}

func parseOptionalID(value string) (*int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	id, err := parseID(value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func auditLog(r *http.Request, auditService *audit.Service, currentUser *auth.CurrentUser, action string, entityType string, entityID int64, metadata map[string]any) {
	var userID *int64
	if currentUser != nil {
		userID = &currentUser.ID
	}
	ip, userAgent := audit.RequestFields(r)
	auditService.Log(r.Context(), audit.Event{
		UserID:     userID,
		Action:     action,
		EntityType: entityType,
		EntityID:   &entityID,
		Metadata:   metadata,
		IP:         ip,
		UserAgent:  userAgent,
	})
}
