// Package app assembles the fully-wired API stack (config, Postgres,
// Redis, TMDB/OMDb clients, repositories, usecases, router). Both entry
// points — cmd/api/main.go (long-running server) and api/index.go (the
// Vercel function) — build from here, so the stack is defined once.
package app

import (
	"context"
	"database/sql"
	"net/http"

	"movie-trivia/internal/config"
	"movie-trivia/internal/cron"
	httphandler "movie-trivia/internal/handler/http"
	"movie-trivia/internal/provider/omdb"
	"movie-trivia/internal/provider/tmdb"
	"movie-trivia/internal/repository/postgres"
	rediscache "movie-trivia/internal/repository/redis"
	"movie-trivia/internal/usecase"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	// Embeds the IANA timezone database so POOL_REFRESH_TZ (e.g.
	// Asia/Jakarta) resolves on platforms without a system tzdata.
	_ "time/tzdata"
)

// App is the fully-wired API stack.
type App struct {
	Config    *config.Config
	Handler   http.Handler
	Scheduler *cron.Scheduler
}

// Build assembles the entire API stack from configuration: Postgres and
// Redis connections, the TMDB/OMDb clients, repositories, caches,
// usecases, and the HTTP router. Construction errors panic — a
// half-wired API is worse than no API (both entry points recover them
// into startup/function errors).
func Build(ctx context.Context) *App {
	cfg, err := config.Load()
	if err != nil {
		panic("config: " + err.Error())
	}

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		panic("postgres: " + err.Error())
	}
	if err := db.PingContext(ctx); err != nil {
		panic("postgres ping: " + err.Error())
	}

	rdb := redis.NewClient(redisOptions(cfg.RedisURL))
	if err := rdb.Ping(ctx).Err(); err != nil {
		panic("redis ping: " + err.Error())
	}

	// Repositories
	sessions := postgres.NewSessionRepo(db)
	dailyGames := postgres.NewDailyGameRepo(db)
	entries := postgres.NewLeaderboardRepo(db)

	// Redis caches + locks
	poolCache := rediscache.NewMasterPoolCache(rdb)
	ratingCache := rediscache.NewRatingCache(rdb)
	posterCache := rediscache.NewPosterCache(rdb)
	dailyLock := rediscache.NewDailyLock(rdb)

	// External API clients
	tmdbClient := tmdb.NewClient(cfg.TMDBAPIKey)
	omdbClient := omdb.NewClient(cfg.OMDBAPIKey)

	// Usecases
	poolUC := usecase.NewPoolUsecase(poolCache, ratingCache, posterCache, tmdbClient, omdbClient)
	gameUC := usecase.NewGameUsecase(poolUC)
	freeplayUC := usecase.NewFreeplayUsecase(sessions, gameUC)
	dailyUC := usecase.NewDailyUsecase(sessions, dailyGames, entries, gameUC, cfg.PoolRefreshLoc)
	leaderboardUC := usecase.NewLeaderboardUsecase(entries, sessions, cfg.PoolRefreshLoc)
	adminUC := usecase.NewAdminUsecase(cfg.AdminPassword, dailyGames)

	// ONE daily job (pool refresh → daily game generation → session GC),
	// triggered lazily by the first request of the day — the 00:00 cron
	// ping from the platform boots the function and starts the chain.
	scheduler := cron.NewScheduler(poolUC, gameUC, dailyGames, dailyLock, sessions, dailyLock, cfg.PoolRefreshLoc)

	router := httphandler.New(dailyUC, freeplayUC, poolUC, leaderboardUC, adminUC, dailyLock, cfg.CronSecret, cfg.AllowedOrigin, cfg.PoolRefreshLoc, scheduler)

	return &App{Config: cfg, Handler: router, Scheduler: scheduler}
}

// redisOptions parses the Redis URL (supports rediss:// for managed
// providers like Upstash).
func redisOptions(url string) *redis.Options {
	opts, err := redis.ParseURL(url)
	if err != nil {
		panic("redis url: " + err.Error())
	}
	return opts
}
