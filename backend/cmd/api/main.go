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
	"movie-trivia/internal/repository/postgres"
	rediscache "movie-trivia/internal/repository/redis"
	"movie-trivia/internal/usecase"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func main() {
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

	// Usecases
	poolUC := usecase.NewPoolUsecase(poolCache, ratingCache)
	gameUC := usecase.NewGameUsecase(poolUC)
	freeplayUC := usecase.NewFreeplayUsecase(sessions, gameUC)
	dailyUC := usecase.NewDailyUsecase(sessions, dailyGames, entries, gameUC, cfg.PoolRefreshLoc)
	leaderboardUC := usecase.NewLeaderboardUsecase(entries, sessions, cfg.PoolRefreshLoc)
	adminUC := usecase.NewAdminUsecase(cfg.AdminPassword, dailyGames)

	// Background jobs: master pool refresh (every 4h) and daily
	// game generation (once per calendar day in POOL_REFRESH_TZ).
	scheduler := cron.NewScheduler()
	scheduler.Start()

	// HTTP
	router := httphandler.New(dailyUC, freeplayUC, poolUC, leaderboardUC, adminUC, dailyLock, cfg.AllowedOrigin, sessions)
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
