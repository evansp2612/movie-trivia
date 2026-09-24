package postgres

import (
	"context"

	"movie-trivia/internal/domain"
)

// PoolRepo would mirror the master pool in Postgres for cache-rebuild
// support. The PRD stores the master pool exclusively in Redis
// (master_pool:batch) with a synchronous refetch failsafe, so this
// remains unimplemented in v1.
type PoolRepo struct{}

func NewPoolRepo() *PoolRepo { return &PoolRepo{} }

func (r *PoolRepo) Save(ctx context.Context, movies []domain.Movie) error {
	return domain.ErrNotImplemented
}

func (r *PoolRepo) List(ctx context.Context) ([]domain.Movie, error) {
	return nil, domain.ErrNotImplemented
}
