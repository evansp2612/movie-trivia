package cron

import (
	"context"
	"time"
)

// runDailyGeneration builds today's fixed 10-round set once per calendar
// day in the POOL_REFRESH_TZ timezone. The dayTick loop detects the
// rollover; daily:gen:lock (SETNX) prevents concurrent generation, and
// UNIQUE(game_date) in Postgres is the final backstop.
func (s *Scheduler) runDailyGeneration(ctx context.Context) {
	lastDay := ""

	check := func() {
		today := time.Now().In(s.loc).Format("2006-01-02")
		if today == lastDay {
			return
		}
		if err := s.generateFor(today); err != nil {
			s.log.Printf("cron: daily generation for %s failed: %v", today, err)
			return // retried on the next tick
		}
		lastDay = today
	}
	check() // prime on startup

	ticker := time.NewTicker(dayTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}

func (s *Scheduler) generateFor(gameDate string) error {
	ctx, cancel := context.WithTimeout(context.Background(), genLockTTL)
	defer cancel()

	acquired, err := s.lock.AcquireGenLock(ctx, genLockTTL)
	if err != nil {
		return err
	}
	if !acquired {
		// Another worker (or a racing API instance) holds the lock;
		// UNIQUE(game_date) means the set already exists.
		return nil
	}
	defer s.lock.ReleaseGenLock(ctx)

	// The pool refresh is midnight-anchored too, but it may still be
	// running (or may have failed) when the first dayTick fires; make
	// sure a fresh pool exists before building the day's rounds.
	s.refreshPool(ctx)

	rounds, err := s.daily.BuildRounds(ctx)
	if err != nil {
		return err
	}
	return s.games.Upsert(ctx, gameDate, rounds)
}
