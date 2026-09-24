package omdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	apiKey string
	http   *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey, http: &http.Client{Timeout: 15 * time.Second}}
}

// RatingByIMDB looks up the IMDb rating by the movie's IMDb ID (tt...),
// resolved beforehand from TMDB's external_ids endpoint.
func (c *Client) RatingByIMDB(ctx context.Context, imdbID string) (float64, error) {
	if imdbID == "" {
		return 0, fmt.Errorf("omdb: empty imdb id")
	}
	u := fmt.Sprintf("https://www.omdbapi.com/?apikey=%s&i=%s", url.QueryEscape(c.apiKey), url.QueryEscape(imdbID))
	var raw struct {
		IMDBRating string `json:"imdbRating"`
		Response   string `json:"Response"`
		Error      string `json:"Error"`
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return 0, err
	}
	if raw.Response != "True" {
		return 0, fmt.Errorf("omdb: %s", raw.Error)
	}
	var r float64
	if _, err := fmt.Sscanf(raw.IMDBRating, "%f", &r); err != nil {
		return 0, fmt.Errorf("omdb: bad rating %q", raw.IMDBRating)
	}
	return r, nil
}
