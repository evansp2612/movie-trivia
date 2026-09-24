package httphandler

import (
	"context"
	"log"
	"net/http"
	"time"

	"movie-trivia/internal/domain"
	"movie-trivia/internal/repository"
	rediscache "movie-trivia/internal/repository/redis"
	"movie-trivia/internal/usecase"
)

// New builds the full API route table (PRD Part 3 §4).
func New(
	daily *usecase.DailyUsecase,
	freeplay *usecase.FreeplayUsecase,
	pool *usecase.PoolUsecase,
	leaderboard *usecase.LeaderboardUsecase,
	admin *usecase.AdminUsecase,
	lock *rediscache.DailyLock,
	allowedOrigin string,
	sessions repository.SessionRepo,
) http.Handler {
	mux := http.NewServeMux()

	dailyH := NewDailyHandler(daily)
	freeplayH := NewFreeplayHandler(freeplay)
	poolH := NewPoolHandler(pool)
	lbH := NewLeaderboardHandler(leaderboard)
	adminH := NewAdminHandler(admin)

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

	// Anonymous player identity via long-lived HTTP-only cookie.
	handler := playerIDMiddleware(mux)

	// Background GC of abandoned Free Play sessions (stale-session timer).
	go runSessionGC(sessions)

	return corsMiddleware(allowedOrigin, handler)
}

func runSessionGC(sessions repository.SessionRepo) {
	ticker := time.NewTicker(time.Hour)
	for range ticker.C {
		n, err := sessions.DeleteStale(context.Background(), domain.ModeFreePlay, 24*time.Hour)
		if err != nil {
			log.Printf("session gc: %v", err)
			continue
		}
		if n > 0 {
			log.Printf("session gc: removed %d stale freeplay sessions", n)
		}
	}
}
