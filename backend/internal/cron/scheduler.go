package cron

import (
	"context"
	"log"
	"sync"
	"time"

	"movie-trivia/internal/domain"
)

// genLockTTL bounds how long daily:gen:lock can be held; generation
// itself takes seconds, so this only guards against a crashed worker.
const genLockTTL = 10 * time.Minute

// staleAge is how old an incomplete Free Play session must be before
// the daily GC sweeps it.
const staleAge = 24 * time.Hour

// healThrottle limits how often the missing-game self-heal may re-run
// the chain (per process).
const healThrottle = 5 * time.Minute

// PoolRefresher regenerates the master pool (*usecase.PoolUsecase is
// the production implementation).
type PoolRefresher interface {
	Refresh(ctx context.Context) ([]domain.Movie, error)
}

// RoundBuilder produces a fresh 10-round variant (*usecase.GameUsecase
// is the production implementation).
type RoundBuilder interface {
	BuildRounds(ctx context.Context) ([]domain.Round, error)
}

// SessionGCRepo sweeps stale sessions (implemented by the postgres
// SessionRepo; only DeleteStale is needed).
type SessionGCRepo interface {
	DeleteStale(ctx context.Context, mode domain.GameMode, maxAge time.Duration) (int64, error)
}

// Scheduler runs the project's ONE daily job: a combined chain of pool
// refresh → daily game generation → stale session GC. On Vercel
// Functions there are no long-lived timers, so the chain is triggered
// lazily by the first request of the day (the 00:00 cron ping); the
// per-date Redis marker guarantees at most one run per calendar day.
type Scheduler struct {
	pool     PoolRefresher
	daily    RoundBuilder
	games    DailyGameStore
	lock     GenLock
	sessions SessionGCRepo
	marker   DayMarker
	loc      *time.Location
	log      *log.Logger

	once        sync.Once // RunIfNeeded checks at most once per process instance
	lastAttempt time.Time // throttles the missing-game self-heal re-runs
}

// DailyGameStore persists and checks the fixed 10-round set
// (implemented by the postgres DailyGameRepo).
type DailyGameStore interface {
	Upsert(ctx context.Context, gameDate string, rounds []domain.Round) error
	Has(ctx context.Context, gameDate string) (bool, error)
}

// GenLock is the lock guarding concurrent daily generation
// (implemented by the redis DailyLock).
type GenLock interface {
	AcquireGenLock(ctx context.Context, ttl time.Duration) (bool, error)
	ReleaseGenLock(ctx context.Context) error
}

// DayMarker claims the once-per-day run (implemented by the redis
// DailyLock with the per-date cron:last_run SETNX key).
type DayMarker interface {
	ClaimDailyRun(ctx context.Context, date string) (bool, error)
}

func NewScheduler(pool PoolRefresher, daily RoundBuilder, games DailyGameStore,
	lock GenLock, sessions SessionGCRepo, marker DayMarker, loc *time.Location) *Scheduler {
	return &Scheduler{
		pool:     pool,
		daily:    daily,
		games:    games,
		lock:     lock,
		sessions: sessions,
		marker:   marker,
		loc:      loc,
		log:      log.Default(),
	}
}

func (s *Scheduler) today() string { return time.Now().In(s.loc).Format("2006-01-02") }

// DailySummary reports what the daily run did (returned by the cron
// endpoint so the ping's response documents the run).
type DailySummary struct {
	PoolCount  int    `json:"pool_count"`
	GameStatus string `json:"game_status"` // "generated" | "exists" | "failed"
	GcRemoved  int64  `json:"gc_removed"`
}

// RunDaily executes the full maintenance chain in order: pool refresh →
// daily game generation → stale session GC. Each step is logged and a
// failing step does not abort the rest (the markers and constraints
// make every step safe to retry on the next day's run).
func (s *Scheduler) RunDaily(ctx context.Context) (DailySummary, error) {
	s.log.Println("cron: daily run started")
	summary := DailySummary{GameStatus: "failed"}

	// 1. Pool refresh: TMDB popular/top_rated + OMDb ratings + textless
	// posters (idempotent; ratings cached 7 days).
	pool, err := s.pool.Refresh(ctx)
	if err != nil {
		s.log.Printf("cron: pool refresh failed: %v", err)
	} else {
		summary.PoolCount = len(pool)
		s.log.Printf("cron: pool refreshed (%d movies)", len(pool))
	}

	// 2. Today's Game of the Day: guarded by daily:gen:lock, with
	// UNIQUE(game_date) as the final backstop.
	gameDate := s.today()
	acquired, err := s.lock.AcquireGenLock(ctx, genLockTTL)
	if err != nil {
		s.log.Printf("cron: gen lock: %v", err)
	} else if acquired {
		rounds, err := s.daily.BuildRounds(ctx)
		if err != nil {
			s.log.Printf("cron: daily generation failed: %v", err)
		} else if err := s.games.Upsert(ctx, gameDate, rounds); err != nil {
			s.log.Printf("cron: daily game upsert: %v", err)
		} else {
			summary.GameStatus = "generated"
			s.log.Printf("cron: daily game for %s generated (%d rounds)", gameDate, len(rounds))
		}
		s.lock.ReleaseGenLock(ctx)
	} else {
		summary.GameStatus = "exists"
		s.log.Printf("cron: daily generation skipped (lock held elsewhere)")
	}

	// 3. Stale session GC: incomplete Free Play sessions older than 24h.
	n, err := s.sessions.DeleteStale(ctx, domain.ModeFreePlay, staleAge)
	if err != nil {
		s.log.Printf("cron: session gc failed: %v", err)
	} else {
		summary.GcRemoved = n
		if n > 0 {
			s.log.Printf("cron: session gc removed %d stale freeplay sessions", n)
		}
	}

	s.log.Printf("cron: daily run finished (%+v)", summary)
	return summary, nil
}

// ForceRun executes the daily chain immediately, bypassing the
// once-per-day marker — used by the protected cron endpoint, where an
// explicit ping means "run now". Every step is idempotent, so this is
// safe even if the chain already ran today. Returns the summary as any
// so the HTTP layer's ForceRunner interface is satisfied directly.
func (s *Scheduler) ForceRun(ctx context.Context) (any, error) {
	return s.RunDaily(ctx)
}

// RunIfNeeded triggers the daily chain if it has not run today. Called
// from the request middleware: the 00:00 cron ping (or the first player
// request of the day) boots the function and kicks the chain off in the
// background — requests never block on it. The sync.Once bounds each
// process instance to a single check.
func (s *Scheduler) RunIfNeeded(ctx context.Context) {
	s.once.Do(func() {
		date := s.today()
		claimed, err := s.marker.ClaimDailyRun(ctx, date)
		if err != nil {
			s.log.Printf("cron: day marker: %v", err)
			return
		}
		if !claimed {
			s.log.Printf("cron: daily run for %s already claimed", date)
			return
		}
		s.startDailyRun()
	})

	// Self-heal: the marker being claimed only proves the chain STARTED
	// today — if it died before generating the game, re-run it (throttled
	// so concurrent requests don't stampede).
	s.healIfGameMissing(ctx)
}

// healIfGameMissing re-runs the daily chain when today's game is missing
// (a midnight run that died before generation), throttled to one attempt
// per healThrottle.
func (s *Scheduler) healIfGameMissing(ctx context.Context) {
	if time.Since(s.lastAttempt) < healThrottle {
		return
	}
	s.lastAttempt = time.Now()

	has, err := s.games.Has(ctx, s.today())
	if err != nil {
		s.log.Printf("cron: game presence check: %v", err)
		return
	}
	if has {
		return
	}
	s.log.Printf("cron: today's game is missing — self-healing re-run")
	s.startDailyRun()
}

func (s *Scheduler) startDailyRun() {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), genLockTTL)
		defer cancel()
		if _, err := s.RunDaily(bgCtx); err != nil {
			s.log.Printf("cron: daily run error: %v", err)
		}
	}()
}

// ForceRunAny is the any-typed wrapper satisfying the HTTP layer's
// ForceRunner interface without the router importing this package's
// summary type.
func (s *Scheduler) ForceRunAny(ctx context.Context) (any, error) {
	return s.ForceRun(ctx)
}
