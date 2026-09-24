package cron

import "log"

// Scheduler runs recurring background jobs: the master pool refresh
// (every 4 hours) and the daily game generation (once per calendar day).
type Scheduler struct {
	// TODO: inject pool usecase, daily usecase, and redis lock client.
}

func NewScheduler() *Scheduler {
	return &Scheduler{}
}

// Start launches all background job loops. It returns immediately;
// jobs run on their own goroutines and should be stopped via ctx.
func (s *Scheduler) Start() {
	log.Println("cron scheduler started")
	// TODO: go s.runPoolRefresh(ctx)   — ticker every 4h
	// TODO: go s.runDailyGeneration(ctx) — check calendar-day rollover in POOL_REFRESH_TZ
}
