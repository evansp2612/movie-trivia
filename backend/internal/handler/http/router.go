package httphandler

import (
	"context"
	"net/http"
	"time"

	rediscache "movie-trivia/internal/repository/redis"
	"movie-trivia/internal/usecase"
)

// DailyRunner triggers and runs the once-per-day maintenance chain.
// Satisfied by cron.Scheduler in main.go: RunIfNeeded is the lazy
// per-request check, ForceRun the explicit synchronous cron trigger.
type DailyRunner interface {
	RunIfNeeded(ctx context.Context)
	ForceRun(ctx context.Context) (any, error)
}

// New builds the full API route table (PRD Part 3 §4).
func New(
	daily *usecase.DailyUsecase,
	freeplay *usecase.FreeplayUsecase,
	pool *usecase.PoolUsecase,
	leaderboard *usecase.LeaderboardUsecase,
	admin *usecase.AdminUsecase,
	lock *rediscache.DailyLock,
	cronSecret string,
	allowedOrigin string,
	loc *time.Location,
	runner DailyRunner,
) http.Handler {
	mux := http.NewServeMux()

	dailyH := NewDailyHandler(daily)
	freeplayH := NewFreeplayHandler(freeplay)
	poolH := NewPoolHandler(pool)
	lbH := NewLeaderboardHandler(leaderboard)
	adminH := NewAdminHandler(admin, loc)
	cronH := NewCronHandler(runner, cronSecret, admin.Password())

	// Pool / auto-complete
	mux.HandleFunc("GET /api/pool/titles", poolH.Titles)

	// Daily
	mux.HandleFunc("GET /api/daily/status", dailyH.Status)
	mux.HandleFunc("POST /api/daily/start", dailyH.Start)
	mux.HandleFunc("GET /api/daily/round/{n}", dailyH.Round)
	mux.HandleFunc("POST /api/daily/round/{n}/answer", dailyH.Answer)
	mux.HandleFunc("GET /api/daily/result", dailyH.Result)
	mux.HandleFunc("POST /api/daily/leaderboard", lbH.Submit)
	mux.HandleFunc("GET /api/daily/leaderboard", lbH.Top10)

	// Free Play
	mux.HandleFunc("POST /api/freeplay/start", freeplayH.Start)
	mux.HandleFunc("GET /api/freeplay/{id}/round/{n}", freeplayH.Round)
	mux.HandleFunc("POST /api/freeplay/{id}/round/{n}/answer", freeplayH.Answer)
	mux.HandleFunc("GET /api/freeplay/{id}/result", freeplayH.Result)

	// Admin (password-gated reveal)
	mux.Handle("GET /api/admin/daily/reveal", AdminMiddleware(admin, lock)(http.HandlerFunc(adminH.Reveal)))

	// Cron trigger (protected; called by the platform cron at 00:00)
	mux.HandleFunc("GET /api/cron/daily", cronH.Daily)

	// Anonymous player identity via long-lived HTTP-only cookie, then the
	// once-per-day maintenance trigger: the first request after midnight
	// (the platform cron ping) boots the daily chain in the background.
	handler := playerIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		runner.RunIfNeeded(r.Context())
		mux.ServeHTTP(w, r)
	}))

	return corsMiddleware(allowedOrigin, handler)
}
