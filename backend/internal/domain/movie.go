package domain

// Movie is a candidate from the unified master pool. Only poster,
// title, year, and rating are ever surfaced to the client — no plot,
// taglines, or cast data.
type Movie struct {
	TMDBID    int    `json:"tmdb_id"`
	Title     string `json:"title"`
	Year      int    `json:"year"`
	PosterURL string `json:"poster_url"`
	// TextlessPosterURL must serialize: it is persisted in daily_games
	// and the Redis pool cache, and it is the image shown (blurred) in
	// blurred-poster rounds. The DTO layer decides what reaches clients.
	TextlessPosterURL string  `json:"textless_poster_url,omitempty"`
	IMDBRating        float64 `json:"imdb_rating"`
}
