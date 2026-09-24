package rediscache

import (
	"context"
	"encoding/json"

	"movie-trivia/internal/domain"

	"github.com/redis/go-redis/v9"
)

type MasterPoolCache struct{ rdb *redis.Client }

func NewMasterPoolCache(rdb *redis.Client) *MasterPoolCache { return &MasterPoolCache{rdb: rdb} }

const masterPoolKey = "master_pool:batch"

// Get returns the current unified candidate batch (up to 80 movies).
// A nil, nil result means cache miss — the caller applies the
// synchronous refetch failsafe.
func (c *MasterPoolCache) Get(ctx context.Context) ([]domain.Movie, bool, error) {
	b, err := c.rdb.Get(ctx, masterPoolKey).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var movies []domain.Movie
	if err := json.Unmarshal(b, &movies); err != nil {
		return nil, false, err
	}
	return movies, true, nil
}

func (c *MasterPoolCache) Set(ctx context.Context, movies []domain.Movie) error {
	b, err := json.Marshal(movies)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, masterPoolKey, b, 0).Err()
}
