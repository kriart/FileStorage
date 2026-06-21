package admin

import "context"

type DashboardRepository interface {
	Dashboard(ctx context.Context) (*Dashboard, error)
}

type Service struct {
	repository DashboardRepository
}

func NewService(repository DashboardRepository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Dashboard(ctx context.Context) (*Dashboard, error) {
	return s.repository.Dashboard(ctx)
}
