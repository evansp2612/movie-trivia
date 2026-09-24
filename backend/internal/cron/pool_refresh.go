package cron

// runPoolRefresh regenerates the unified master pool every 4 hours:
// TMDB /movie/popular + /movie/top_rated (pages 1-2), English only,
// dedupe by tmdb_id, cap at 80, then OMDb ratings + textless posters
// with a 50ms delay between iterations.
func (s *Scheduler) runPoolRefresh() {
	// TODO: ticker; delegate to pool usecase; populate
	// master_pool:batch and rating:{tmdb_id} caches.
}
