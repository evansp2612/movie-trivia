package cron

import (
	"context"
	"log"
	"time"

	"movie-trivia/internal/domain"
	"movie-trivia/internal/usecase"
)

// dayTick is how often the scheduler checks for a calendar-day rollover
// in the configured timezone. It is deliberately much finer than 24h so
// the day's game is generated shortly after local midnight, and so
// clock skew or a restart mid-day cannot skip generation.
const dayTick = 15 * time.Minute

// genLockTTL bounds how long daily:gen:lock can be held; generation
// itself takes seconds, so this only guards against a crashed worker.
const genLockTTL = 10 * time.Minute

// Scheduler runs recurring background jobs: the master pool refresh
// (just after local midnight daily) and the daily game generation
// (once per calendar day, from the freshly refreshed pool).
type Scheduler struct {
	pool  *usecase.PoolUsecase
	daily *usecase.GameUsecase
	games DailyGameStore
	lock  GenLock
	loc   *time.Location
	log   *log.Logger
}

// DailyGameStore persists the fixed 10-round set (implemented by the
// postgres DailyGameRepo).
type DailyGameStore interface {
	Upsert(ctx context.Context, gameDate string, rounds []domain.Round) error
}

// GenLock is the lock guarding concurrent daily generation
// (implemented by the redis DailyLock).
type GenLock interface {
	AcquireGenLock(ctx context.Context, ttl time.Duration) (bool, error)
	ReleaseGenLock(ctx context.Context) error
}

func NewScheduler(pool *usecase.PoolUsecase, daily *usecase.GameUsecase, games DailyGameStore, lock GenLock, loc *time.Location) *Scheduler {
	return &Scheduler{
		pool:  pool,
		daily: daily,
		games: games,
		lock:  lock,
		loc:   loc,
		log:   log.Default(),
	}
}

// Start launches all background job loops. It returns immediately;
// the loops run on their own goroutines and stop when ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	s.log.Println("cron scheduler started")
	go s.runPoolRefresh(ctx)
	go s.runDailyGeneration(ctx)
}
