package domain

// Movie is a candidate from the unified master pool. Only poster,
// title, year, and rating are ever surfaced to the client — no plot,
// taglines, or cast data.
type Movie struct {
	TMDBID            int     `json:"tmdb_id"`
	Title             string  `json:"title"`
	Year              int     `json:"year"`
	PosterURL         string  `json:"poster_url"`
	TextlessPosterURL string  `json:"-"`
	IMDBRating        float64 `json:"imdb_rating"`
}
