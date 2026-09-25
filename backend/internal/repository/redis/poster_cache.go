package rediscache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// PosterCache stores textless poster paths per TMDB ID so repeat pool
// refreshes skip the TMDB images API entirely.
type PosterCache struct{ rdb *redis.Client }

func NewPosterCache(rdb *redis.Client) *PosterCache { return &PosterCache{rdb: rdb} }

const posterTTL = 7 * 24 * time.Hour

func posterKey(tmdbID int) string { return "textless:" + itoa(tmdbID) }

// Get returns the cached textless poster path. found=false on miss.
// An empty stored value is a cached "no textless poster exists" result.
func (c *PosterCache) Get(ctx context.Context, tmdbID int) (string, bool, error) {
	v, err := c.rdb.Get(ctx, posterKey(tmdbID)).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// Set caches the textless poster path (may be empty when the movie has
// none) so the daily chain never re-queries TMDB for it within the TTL.
func (c *PosterCache) Set(ctx context.Context, tmdbID int, path string) error {
	return c.rdb.Set(ctx, posterKey(tmdbID), path, posterTTL).Err()
}
