// Command api wires together configuration, Postgres, Redis, the
// TMDB/OMDB clients, the cron scheduler, and the HTTP router.
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"movie-trivia/internal/config"
	"movie-trivia/internal/cron"
	httphandler "movie-trivia/internal/handler/http"
	"movie-trivia/internal/provider/omdb"
	"movie-trivia/internal/provider/tmdb"
	"movie-trivia/internal/repository/postgres"
	rediscache "movie-trivia/internal/repository/redis"
	"movie-trivia/internal/usecase"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Load backend/.env (if present). Real environment variables take
	// precedence, so docker-compose's env_file/environment blocks still win.
	// Each path is tried independently: godotenv.Load aborts on the first
	// missing file, which would skip the one that exists.
	loaded := false
	for _, path := range []string{".env", "../.env"} {
		if err := godotenv.Load(path); err == nil {
			loaded = true
			break
		}
	}
	if !loaded {
		log.Printf("config: no .env loaded (falling back to OS environment)")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("postgres ping: %v", err)
	}

	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis url: %v", err)
	}
	rdb := redis.NewClient(opts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis ping: %v", err)
	}

	// Repositories
	sessions := postgres.NewSessionRepo(db)
	dailyGames := postgres.NewDailyGameRepo(db)
	entries := postgres.NewLeaderboardRepo(db)

	// Redis caches
	poolCache := rediscache.NewMasterPoolCache(rdb)
	ratingCache := rediscache.NewRatingCache(rdb)
	dailyLock := rediscache.NewDailyLock(rdb)

	// External API clients
	tmdbClient := tmdb.NewClient(cfg.TMDBAPIKey)
	omdbClient := omdb.NewClient(cfg.OMDBAPIKey)

	// Usecases
	poolUC := usecase.NewPoolUsecase(poolCache, ratingCache, tmdbClient, omdbClient)
	gameUC := usecase.NewGameUsecase(poolUC)
	freeplayUC := usecase.NewFreeplayUsecase(sessions, gameUC)
	dailyUC := usecase.NewDailyUsecase(sessions, dailyGames, entries, gameUC, cfg.PoolRefreshLoc)
	leaderboardUC := usecase.NewLeaderboardUsecase(entries, sessions, cfg.PoolRefreshLoc)
	adminUC := usecase.NewAdminUsecase(cfg.AdminPassword, dailyGames)

	// ONE daily job (pool refresh → daily game generation → session GC),
	// triggered lazily by the first request of the day — the 00:00 cron
	// ping from the platform boots the function and starts the chain.
	scheduler := cron.NewScheduler(poolUC, gameUC, dailyGames, dailyLock, sessions, dailyLock, cfg.PoolRefreshLoc)
	scheduler.RunIfNeeded(ctx)

	// HTTP
	router := httphandler.New(dailyUC, freeplayUC, poolUC, leaderboardUC, adminUC, dailyLock, cfg.CronSecret, cfg.AllowedOrigin, cfg.PoolRefreshLoc, scheduler)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("api listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
