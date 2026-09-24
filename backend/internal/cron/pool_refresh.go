package cron

import (
	"context"
	"time"
)

// runPoolRefresh regenerates the unified master pool anchored to local
// midnight in POOL_REFRESH_TZ: once immediately on startup (so a fresh
// deployment has a pool before the first request), then again just
// after each midnight so the day's game is built from a fresh pool.
// It delegates to the pool usecase, which also populates the
// rating:{tmdb_id} cache.
func (s *Scheduler) runPoolRefresh(ctx context.Context) {
	s.refreshPool(ctx) // prime on startup

	for {
		untilMidnight := timeUntilNextMidnight(s.loc)
		select {
		case <-ctx.Done():
			return
		case <-time.After(untilMidnight):
			s.refreshPool(ctx)
		}
	}
}

// timeUntilNextMidnight returns the duration from now until the next
// local midnight (in POOL_REFRESH_TZ).
func timeUntilNextMidnight(loc *time.Location) time.Duration {
	return timeUntilNextMidnightAt(time.Now(), loc)
}

func timeUntilNextMidnightAt(now time.Time, loc *time.Location) time.Duration {
	now = now.In(loc)
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	if !now.Before(midnight) {
		midnight = midnight.AddDate(0, 0, 1)
	}
	return midnight.Sub(now)
}

func (s *Scheduler) refreshPool(ctx context.Context) {
	started := time.Now()
	if _, err := s.pool.Refresh(ctx); err != nil {
		s.log.Printf("cron: pool refresh failed after %s: %v", time.Since(started), err)
		return
	}
	s.log.Printf("cron: pool refresh completed in %s", time.Since(started))
}
