package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const baseURL = "https://api.themoviedb.org/3"

type Client struct {
	apiKey    string
	v4        bool
	poolPages int
	http      *http.Client
}

// NewClient takes poolPages: how many pages of each discover set feed
// the candidate pool (the pool's size knob — see FetchPool).
func NewClient(apiKey string, poolPages int) *Client {
	v4 := len(apiKey) > 60 && len(apiKey) >= 2 && apiKey[:2] == "ey"
	return &Client{apiKey: apiKey, v4: v4, poolPages: poolPages, http: &http.Client{Timeout: 15 * time.Second}}
}

type Movie struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	ReleaseDate  string `json:"release_date"`
	PosterPath   string `json:"poster_path"`
	OriginalLang string `json:"original_language"`
}

type pagedResponse struct {
	Results []Movie `json:"results"`
}

// The pool's discover queries share one filter and differ only in
// sort order: with_original_language=en is TMDB's authoritative
// production-language filter, vote_count.gte=500 keeps both sets to
// movies with meaningful votes (excludes brand-new releases with
// incomplete metadata), and adult content is excluded.
const discoverFilter = "with_original_language=en&vote_count.gte=500&include_adult=false"

var discoverSorts = []string{"popularity.desc", "vote_average.desc"}

// FetchPool queries /discover/movie (c.poolPages pages per sort set,
// filter applied server-side by TMDB). The two sets are fetched
// concurrently and their results interleaved so both sources
// contribute evenly to the pool.
func (c *Client) FetchPool(ctx context.Context) ([]Movie, error) {
	perSet := make([][]Movie, len(discoverSorts))
	var wg sync.WaitGroup
	errs := make([]error, len(discoverSorts))

	for si, sortBy := range discoverSorts {
		wg.Add(1)
		go func(si int, sortBy string) {
			defer wg.Done()
			var out []Movie
			for page := 1; page <= c.poolPages; page++ {
				var res pagedResponse
				url := fmt.Sprintf("%s/discover/movie?page=%d&sort_by=%s&%s",
					baseURL, page, sortBy, discoverFilter)
				if err := c.get(ctx, url, &res); err != nil {
					errs[si] = fmt.Errorf("tmdb: discover[%s] page %d: %w", sortBy, page, err)
					return
				}
				out = append(out, res.Results...)
			}
			perSet[si] = out
		}(si, sortBy)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	// Round-robin interleave the two sets, deduping by TMDB ID.
	var out []Movie
	seen := map[int]bool{}
	for i := 0; i < c.poolPages*20; i++ {
		for _, set := range perSet {
			if i < len(set) {
				m := set[i]
				if !seen[m.ID] {
					seen[m.ID] = true
					out = append(out, m)
				}
			}
		}
	}
	return out, nil
}

type ExternalIDs struct {
	IMDBID string `json:"imdb_id"`
}

// FetchIMDBID resolves the movie's IMDb ID (tt...) via TMDB's
// /movie/{id}/external_ids. Callers use it for OMDb rating lookups.
func (c *Client) FetchIMDBID(ctx context.Context, movieID int) (string, error) {
	var res ExternalIDs
	url := fmt.Sprintf("%s/movie/%d/external_ids", baseURL, movieID)
	if err := c.get(ctx, url, &res); err != nil {
		return "", fmt.Errorf("tmdb: external ids for %d: %w", movieID, err)
	}
	return res.IMDBID, nil
}

type Images struct {
	Posters []struct {
		FilePath string  `json:"file_path"`
		ISO639_1 *string `json:"iso_639_1"`
	} `json:"posters"`
}

// FetchTextlessPoster returns the poster path with no embedded language
// (iso_639_1 == null) for the blurred-poster rounds.
func (c *Client) FetchTextlessPoster(ctx context.Context, movieID int) (string, error) {
	var res Images
	url := fmt.Sprintf("%s/movie/%d/images", baseURL, movieID)
	if err := c.get(ctx, url, &res); err != nil {
		return "", fmt.Errorf("tmdb: images for %d: %w", movieID, err)
	}
	for _, p := range res.Posters {
		if p.ISO639_1 == nil {
			return p.FilePath, nil
		}
	}
	return "", nil
}

func (c *Client) get(ctx context.Context, url string, out any) error {
	if !c.v4 {
		// v3 API key goes in the query string.
		if strings.Contains(url, "?") {
			url += "&api_key=" + c.apiKey
		} else {
			url += "?api_key=" + c.apiKey
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if c.v4 {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ImageURL builds a poster URL from a /movie/... path.
func ImageURL(path string) string {
	if path == "" {
		return ""
	}
	return "https://image.tmdb.org/t/p/w500" + path
}
