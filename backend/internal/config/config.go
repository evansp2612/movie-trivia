package config

import (
	"errors"
	"os"
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

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
