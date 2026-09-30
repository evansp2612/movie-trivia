package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	TMDBAPIKey     string
	OMDBAPIKey     string
	DatabaseURL    string
	RedisURL       string
	AdminPassword  string
	CronSecret     string
	Port           string
	AllowedOrigin  string
	PoolRefreshLoc *time.Location
	// PoolPages is how many pages of each TMDB discover set feed the
	// master pool — the pool's size knob (~20 movies × sets × pages).
	PoolPages int
}

// Load reads configuration from the environment. TMDB_API_KEY and
// OMDB_API_KEY are required; everything else has a usable default.
func Load() (*Config, error) {
	cfg := &Config{
		TMDBAPIKey:    os.Getenv("TMDB_API_KEY"),
		OMDBAPIKey:    os.Getenv("OMDB_API_KEY"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		RedisURL:      os.Getenv("REDIS_URL"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		CronSecret:    os.Getenv("CRON_SECRET"),
		Port:          os.Getenv("PORT"),
		AllowedOrigin: os.Getenv("ALLOWED_ORIGIN"),
	}

	if cfg.TMDBAPIKey == "" || cfg.OMDBAPIKey == "" {
		return nil, errors.New("config: TMDB_API_KEY and OMDB_API_KEY must be set")
	}

	tz := getenv("POOL_REFRESH_TZ", "UTC")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, err
	}
	cfg.PoolRefreshLoc = loc

	pages, err := getenvInt("TMDB_POOL_PAGES", 6)
	if err != nil {
		return nil, err
	}
	if pages < 1 || pages > 20 {
		return nil, fmt.Errorf("config: TMDB_POOL_PAGES must be between 1 and 20, got %d", pages)
	}
	cfg.PoolPages = pages

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getenvInt parses an integer env var; unset → fallback, unparsable →
// error (fail fast like POOL_REFRESH_TZ).
func getenvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s must be an integer, got %q", key, v)
	}
	return n, nil
}
