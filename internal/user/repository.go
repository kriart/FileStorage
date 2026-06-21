package user

import (
	"context"

	"file-storage-server/internal/domain"
)

type Repository interface {
	Create(ctx context.Context, params CreateUserParams) (*domain.User, error)
	GetByID(ctx context.Context, id int64) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	List(ctx context.Context) ([]domain.User, error)
	UpdateRole(ctx context.Context, params UpdateUserRoleParams) (*domain.User, error)
}
