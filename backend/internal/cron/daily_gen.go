package cron

// runDailyGeneration builds today's fixed 10-round set once per calendar
// day in the POOL_REFRESH_TZ timezone. Concurrency is guarded by the
// daily:gen:lock SETNX lock; UNIQUE(game_date) in Postgres is the final
// backstop against duplicate generation.
func (s *Scheduler) runDailyGeneration() {
	// TODO: detect day rollover; acquire daily:gen:lock; delegate
	// to daily usecase; write to daily_games.
}
