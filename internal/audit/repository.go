package audit

import (
	"context"
	"encoding/json"
)

type Event struct {
	UserID     *int64
	Action     string
	EntityType string
	EntityID   *int64
	Metadata   map[string]any
	IP         string
	UserAgent  string
}

type Repository interface {
	Log(ctx context.Context, event Event) error
}

func metadataJSON(metadata map[string]any) ([]byte, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	return json.Marshal(metadata)
}
