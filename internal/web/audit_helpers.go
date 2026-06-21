package web

import (
	"net/http"

	"file-storage-server/internal/audit"
)

func (h *Handler) log(r *http.Request, userID int64, action string, entityType string, entityID int64, metadata map[string]any) {
	h.logPtr(r, &userID, action, entityType, &entityID, metadata)
}

func (h *Handler) logPtr(r *http.Request, userID *int64, action string, entityType string, entityID *int64, metadata map[string]any) {
	ip, userAgent := audit.RequestFields(r)
	h.audit.Log(r.Context(), audit.Event{
		UserID:     userID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Metadata:   metadata,
		IP:         ip,
		UserAgent:  userAgent,
	})
}
