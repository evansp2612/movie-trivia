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
	// MaxPoolSize caps the unified master pool at 80 unique movies.
	MaxPoolSize = 80
	// generationDelay is the sleep between movie iterations to stay
	// under TMDB's 40 req/s rate limit (50ms per iteration).
	generationDelay = 50 * time.Millisecond
)

// PoolUsecase serves the unified master pool. On a Redis cache miss it
// applies the emergency failsafe: block the request and synchronously
// regenerate the 80-movie batch.
type PoolUsecase struct {
	cache   *rediscache.MasterPoolCache
	ratings *rediscache.RatingCache
	tmdb    *tmdb.Client
	omdb    *omdb.Client
	mu      sync.Mutex
}

func NewPoolUsecase(cache *rediscache.MasterPoolCache, ratings *rediscache.RatingCache,
	tmdbClient *tmdb.Client, omdbClient *omdb.Client) *PoolUsecase {
	return &PoolUsecase{
		cache:   cache,
		ratings: ratings,
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
// (pages 1-2, English only, deduped by tmdb_id) from the provider, then
// OMDb ratings and textless posters for the first 80 valid candidates.
// Movies that fail rating lookup are skipped (higher/lower rounds need
// a valid rating), so the loop walks beyond 80 candidates if necessary.
func (u *PoolUsecase) Refresh(ctx context.Context) ([]domain.Movie, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	candidates, err := u.tmdb.FetchPool(ctx)
	if err != nil {
		return nil, fmt.Errorf("pool: fetch candidates: %w", err)
	}

	movies := make([]domain.Movie, 0, MaxPoolSize)
	for i, c := range candidates {
		if len(movies) >= MaxPoolSize {
			break
		}
		if i > 0 {
			time.Sleep(generationDelay)
		}

		rating, ok, err := u.ratings.Get(ctx, c.ID)
		if err != nil {
			return nil, fmt.Errorf("pool: rating cache %d: %w", c.ID, err)
		}
		if !ok {
			// Resolve the movie's IMDb ID via TMDB, then rate it on OMDb
			// by that ID — OMDb's direct TMDB-ID mapping is unreliable.
			imdbID, err := u.tmdb.FetchIMDBID(ctx, c.ID)
			if err != nil {
				log.Printf("pool: skipping tmdb %d (%s): %v", c.ID, c.Title, err)
				continue
			}
			rating, err = u.omdb.RatingByIMDB(ctx, imdbID)
			if err != nil {
				log.Printf("pool: skipping tmdb %d (%s): %v", c.ID, c.Title, err)
				continue
			}
			if err := u.ratings.Set(ctx, c.ID, rating); err != nil {
				return nil, fmt.Errorf("pool: cache rating %d: %w", c.ID, err)
			}
		}

		year := 0
		if len(c.ReleaseDate) >= 4 {
			fmt.Sscanf(c.ReleaseDate[:4], "%d", &year)
		}
		textless, err := u.tmdb.FetchTextlessPoster(ctx, c.ID)
		if err != nil {
			log.Printf("pool: tmdb %d (%s): textless poster: %v", c.ID, c.Title, err)
		}

		movies = append(movies, domain.Movie{
			TMDBID:            c.ID,
			Title:             c.Title,
			Year:              year,
			PosterURL:         tmdb.ImageURL(c.PosterPath),
			TextlessPosterURL: tmdb.ImageURL(textless),
			IMDBRating:        rating,
		})
	}

	if len(movies) == 0 {
		return nil, domain.ErrPoolEmpty
	}
	if len(movies) < MaxPoolSize {
		log.Printf("pool: only %d/%d movies valid this cycle", len(movies), MaxPoolSize)
	}

	if err := u.cache.Set(ctx, movies); err != nil {
		return nil, fmt.Errorf("pool: cache batch: %w", err)
	}
	log.Printf("pool: refreshed master pool with %d movies", len(movies))
	return movies, nil
}

// Titles returns the lightweight title list powering the frontend
// auto-complete search (GET /api/pool/titles).
func (u *PoolUsecase) Titles(ctx context.Context) ([]string, error) {
	movies, err := u.Candidates(ctx)
	if err != nil {
		return nil, err
	}
	titles := make([]string, 0, len(movies))
	for _, m := range movies {
		titles = append(titles, m.Title)
	}
	return titles, nil
}
