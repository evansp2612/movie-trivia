package usecase

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"movie-trivia/internal/domain"
	"movie-trivia/internal/provider/omdb"
	"movie-trivia/internal/provider/tmdb"
	rediscache "movie-trivia/internal/repository/redis"
)

const (
	// MaxPoolSize caps the unified master pool at 160 unique movies —
	// a raw candidate store: movies with missing ratings or textless
	// posters are included, and BuildRounds filters per round type.
	MaxPoolSize = 160
	// refreshWorkers is the bounded concurrency for per-movie detail
	// fetches (external_ids, OMDb rating, images).
	refreshWorkers = 8
	// tmdbRateLimit keeps TMDB calls (~20/s) safely under its 40 req/s
	// limit regardless of worker count. OMDb is daily-count limited,
	// not rate limited, so it is not throttled.
	tmdbRateLimit = 20 * time.Millisecond
)

// limiter admits TMDB calls at a fixed interval (token bucket).
type limiter struct {
	ticker *time.Ticker
}

func newLimiter(interval time.Duration) *limiter {
	return &limiter{ticker: time.NewTicker(interval)}
}

func (l *limiter) wait(ctx context.Context) {
	select {
	case <-l.ticker.C:
	case <-ctx.Done():
	}
}

func (l *limiter) stop() { l.ticker.Stop() }

// PoolUsecase serves the unified master pool. On a Redis cache miss it
// applies the emergency failsafe: block the request and synchronously
// regenerate the batch.
type PoolUsecase struct {
	cache   *rediscache.MasterPoolCache
	ratings *rediscache.RatingCache
	posters *rediscache.PosterCache
	tmdb    *tmdb.Client
	omdb    *omdb.Client
	mu      sync.Mutex
}

func NewPoolUsecase(cache *rediscache.MasterPoolCache, ratings *rediscache.RatingCache,
	posters *rediscache.PosterCache, tmdbClient *tmdb.Client, omdbClient *omdb.Client) *PoolUsecase {
	return &PoolUsecase{
		cache:   cache,
		ratings: ratings,
		posters: posters,
		tmdb:    tmdbClient,
		omdb:    omdbClient,
	}
}

// Candidates returns the current master pool batch. A cache miss
// triggers the synchronous regeneration failsafe before returning.
func (u *PoolUsecase) Candidates(ctx context.Context) ([]domain.Movie, error) {
	movies, ok, err := u.cache.Get(ctx)
	if err != nil {
		return nil, err
	}
	if ok {
		return movies, nil
	}
	log.Println("master pool cache miss: regenerating synchronously (v1 failsafe)")
	if _, err := u.Refresh(ctx); err != nil {
		return nil, err
	}
	movies, ok, err = u.cache.Get(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrPoolEmpty
	}
	return movies, nil
}

// Refresh regenerates the unified master pool: TMDB popular + top_rated
// (pages 1-4, English only, deduped by tmdb_id), then per-movie details
// (IMDb rating via OMDb, textless poster via TMDB images) fetched by a
// bounded worker pool. Movies are kept even when details fail (zero
// values) — BuildRounds filters by round-type suitability. Output
// preserves candidate order.
func (u *PoolUsecase) Refresh(ctx context.Context) ([]domain.Movie, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	started := time.Now()

	candidates, err := u.tmdb.FetchPool(ctx)
	if err != nil {
		return nil, fmt.Errorf("pool: fetch candidates: %w", err)
	}
	if len(candidates) > MaxPoolSize {
		candidates = candidates[:MaxPoolSize]
	}

	lim := newLimiter(tmdbRateLimit)
	defer lim.stop()

	results := make([]domain.Movie, len(candidates))
	var wg sync.WaitGroup
	jobs := make(chan int)
	// errCount tracks detail failures; workers keep the movie anyway.
	var errCount int
	var errMu sync.Mutex

	for w := 0; w < refreshWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				m := u.buildMovie(ctx, lim, candidates[i])
				if m.degraded {
					errMu.Lock()
					errCount++
					errMu.Unlock()
				}
				results[i] = m.movie
			}
		}()
	}
	for i := range candidates {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	movies := make([]domain.Movie, 0, len(results))
	for _, m := range results {
		if m.TMDBID != 0 {
			movies = append(movies, m)
		}
	}

	if len(movies) == 0 {
		return nil, domain.ErrPoolEmpty
	}
	if errCount > 0 {
		log.Printf("pool: %d/%d movies have degraded details (no rating and/or textless poster)", errCount, len(movies))
	}
	if len(movies) < MaxPoolSize {
		log.Printf("pool: %d/%d candidates kept this cycle", len(movies), MaxPoolSize)
	}

	if err := u.cache.Set(ctx, movies); err != nil {
		return nil, fmt.Errorf("pool: cache batch: %w", err)
	}
	log.Printf("pool: refreshed master pool with %d movies in %s", len(movies), time.Since(started).Round(time.Millisecond))
	return movies, nil
}

type buildResult struct {
	movie    domain.Movie
	degraded bool
}

// buildMovie assembles one pool movie: cached-or-fetched IMDb rating,
// textless poster, year. Failures degrade the movie (zero values) but
// never drop it — suitability filtering is BuildRounds' job.
func (u *PoolUsecase) buildMovie(ctx context.Context, lim *limiter, c tmdb.Movie) buildResult {
	rating := 0.0
	if r, ok, err := u.ratings.Get(ctx, c.ID); err != nil {
		log.Printf("pool: rating cache %d: %v", c.ID, err)
	} else if ok {
		rating = r
	} else {
		// Resolve the IMDb ID via TMDB, then rate on OMDb by that ID —
		// OMDb's direct TMDB-ID mapping is unreliable.
		lim.wait(ctx)
		imdbID, err := u.tmdb.FetchIMDBID(ctx, c.ID)
		if err != nil {
			log.Printf("pool: tmdb %d (%s): external ids: %v", c.ID, c.Title, err)
		} else if r, err := u.omdb.RatingByIMDB(ctx, imdbID); err != nil {
			log.Printf("pool: omdb %d (%s, %s): %v", c.ID, c.Title, imdbID, err)
		} else {
			rating = r
			if err := u.ratings.Set(ctx, c.ID, rating); err != nil {
				log.Printf("pool: cache rating %d: %v", c.ID, err)
			}
		}
	}

	textless, ok, err := u.posters.Get(ctx, c.ID)
	if err != nil {
		log.Printf("pool: poster cache %d: %v", c.ID, err)
	}
	if err != nil || !ok {
		lim.wait(ctx)
		textless, err = u.tmdb.FetchTextlessPoster(ctx, c.ID)
		if err != nil {
			log.Printf("pool: tmdb %d (%s): textless poster: %v", c.ID, c.Title, err)
		} else if err := u.posters.Set(ctx, c.ID, textless); err != nil {
			log.Printf("pool: cache poster %d: %v", c.ID, err)
		}
	}

	year := 0
	if len(c.ReleaseDate) >= 4 {
		fmt.Sscanf(c.ReleaseDate[:4], "%d", &year)
	}
	m := domain.Movie{
		TMDBID:            c.ID,
		Title:             c.Title,
		Year:              year,
		PosterURL:         tmdb.ImageURL(c.PosterPath),
		TextlessPosterURL: tmdb.ImageURL(textless),
		IMDBRating:        rating,
	}
	return buildResult{movie: m, degraded: m.IMDBRating == 0 || m.TextlessPosterURL == ""}
}

// Titles returns the lightweight title list powering the frontend
// auto-complete search (GET /api/pool/titles).
// PoolTitle is one auto-complete entry: title plus release year, so
// the dropdown can disambiguate remakes and show "Title (Year)".
type PoolTitle struct {
	Title string `json:"title"`
	Year  int    `json:"year"`
}

// Titles returns the lightweight auto-complete list (GET /api/pool/titles).
func (u *PoolUsecase) Titles(ctx context.Context) ([]PoolTitle, error) {
	movies, err := u.Candidates(ctx)
	if err != nil {
		return nil, err
	}
	titles := make([]PoolTitle, 0, len(movies))
	for _, m := range movies {
		titles = append(titles, PoolTitle{Title: m.Title, Year: m.Year})
	}
	return titles, nil
}
