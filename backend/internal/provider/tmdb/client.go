package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const baseURL = "https://api.themoviedb.org/3"

type Client struct {
	apiKey string
	http   *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey, http: &http.Client{Timeout: 15 * time.Second}}
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

// FetchPool calls /movie/popular and /movie/top_rated, pages 1-2,
// keeping English-language movies only. The caller dedupes by TMDB ID
// and enforces rate limiting (50ms between OMDb/image calls).
func (c *Client) FetchPool(ctx context.Context) ([]Movie, error) {
	var out []Movie
	seen := map[int]bool{}
	for _, list := range []string{"popular", "top_rated"} {
		for page := 1; page <= 2; page++ {
			var res pagedResponse
			url := fmt.Sprintf("%s/movie/%s?page=%d&language=en-US", baseURL, list, page)
			if err := c.get(ctx, url, &res); err != nil {
				return nil, fmt.Errorf("tmdb: %s page %d: %w", list, page, err)
			}
			for _, m := range res.Results {
				if m.OriginalLang == "en" && !seen[m.ID] {
					seen[m.ID] = true
					out = append(out, m)
				}
			}
		}
	}
	return out, nil
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
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
