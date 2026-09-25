package rediscache

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type RatingCache struct{ rdb *redis.Client }

func NewRatingCache(rdb *redis.Client) *RatingCache { return &RatingCache{rdb: rdb} }

// ratingTTL is 7 days: IMDb ratings drift slowly, so the daily pool
// refresh re-fetches only new pool entrants (~10-40 OMDb calls/day
// steady state) instead of the whole pool.
const ratingTTL = 7 * 24 * time.Hour

// Get returns the cached IMDb rating for key rating:{tmdbID}.
func (c *RatingCache) Get(ctx context.Context, tmdbID int) (float64, bool, error) {
	b, err := c.rdb.Get(ctx, ratingKey(tmdbID)).Bytes()
	if err == redis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	var rating float64
	if err := json.Unmarshal(b, &rating); err != nil {
		return 0, false, err
	}
	return rating, true, nil
}

func (c *RatingCache) Set(ctx context.Context, tmdbID int, rating float64) error {
	b, _ := json.Marshal(rating)
	return c.rdb.Set(ctx, ratingKey(tmdbID), b, ratingTTL).Err()
}

func ratingKey(tmdbID int) string {
	return "rating:" + itoa(tmdbID)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
