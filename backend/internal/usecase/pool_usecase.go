package usecase

import (
	"context"

	"movie-trivia/internal/domain"
	rediscache "movie-trivia/internal/repository/redis"
)

// PoolUsecase serves the unified master pool. On a Redis cache miss it
// applies the v1 emergency failsafe: block the request and synchronously
// regenerate the 80-movie batch (an acknowledged latency/quota risk).
type PoolUsecase struct {
	cache   *rediscache.MasterPoolCache
	ratings *rediscache.RatingCache
	// TODO: inject TMDB/OMDB providers for the synchronous failsafe.
}

func NewPoolUsecase(cache *rediscache.MasterPoolCache, ratings *rediscache.RatingCache) *PoolUsecase {
	return &PoolUsecase{cache: cache, ratings: ratings}
}

// Candidates returns the current master pool batch.
func (u *PoolUsecase) Candidates(ctx context.Context) ([]domain.Movie, error) {
	movies, ok, err := u.cache.Get(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		// TODO failsafe: synchronously fetch a fresh 80-movie batch
		// (TMDB popular/top_rated pg 1-2 + OMDb ratings + textless
		// posters, 50ms delay per iteration), cache it, and return it.
		return nil, domain.ErrPoolEmpty
	}
	return movies, nil
}

// Titles returns the lightweight title list powering the frontend
// auto-complete search (GET /api/pool/titles).
func (u *PoolUsecase) Titles(ctx context.Context) ([]string, error) {
	movies, err := u.Candidates(ctx)
	if err != nil {
		return nil, err
	}
	titles := make([]string, 0, len(movies))
	for _, m := range movies {
		titles = append(titles, m.Title)
	}
	return titles, nil
}
