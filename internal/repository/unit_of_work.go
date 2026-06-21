package repository

import "context"

type UnitOfWork interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
