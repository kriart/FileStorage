package audit

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
)

type Service struct {
	repository Repository
	logger     *slog.Logger
}

func NewService(repository Repository, logger *slog.Logger) *Service {
	return &Service{repository: repository, logger: logger}
}

func (s *Service) Log(ctx context.Context, event Event) {
	if s == nil || s.repository == nil || event.Action == "" || event.EntityType == "" {
		return
	}
	if err := s.repository.Log(ctx, event); err != nil && s.logger != nil {
		s.logger.Warn("write audit log", "error", err, "action", event.Action, "entity_type", event.EntityType)
	}
}

func RequestFields(r *http.Request) (ip string, userAgent string) {
	ip = strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if ip != "" {
		ip, _, _ = strings.Cut(ip, ",")
		ip = strings.TrimSpace(ip)
	}
	if ip == "" {
		ip = strings.TrimSpace(r.Header.Get("X-Real-IP"))
	}
	if ip == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			ip = host
		} else {
			ip = r.RemoteAddr
		}
	}
	return ip, r.UserAgent()
}

func Int64Ptr(value int64) *int64 {
	return &value
}
