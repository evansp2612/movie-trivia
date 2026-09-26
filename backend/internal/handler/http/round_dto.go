package httphandler

import (
	"movie-trivia/internal/domain"
)

// publicRound builds the client-facing payload for a round.
//
// Spoiler rules (answers stay server-side):
//   - higher/lower: ratings are redacted (the rating comparison IS the
//     answer); title/year/poster stay, as the PRD shows them openly.
//   - guess_the_year: the release year is redacted (it IS the answer);
//     title/poster/rating-bound info stays. Ratings are redacted too —
//     the client never needs them pre-answer.
//   - blurred_poster: only the textless poster image is sent. Title,
//     year, tmdb_id, standard poster, and rating all identify the movie
//     and must not reach the client before the guess.
//
// The stored round (daily_games / session) is never mutated; admin
// reveal keeps the full data.
func publicRound(r *domain.Round, attempts int) any {
	if r == nil {
		return nil
	}

	if r.Type == domain.RoundBlurred {
		posterURL := ""
		if len(r.Movies) > 0 {
			posterURL = r.Movies[0].TextlessPosterURL
		}
		return blurredRoundPayload{
			Index:     r.Index,
			Type:      r.Type,
			PosterURL: posterURL,
			Attempts:  attempts,
		}
	}

	cp := *r
	cp.Movies = make([]domain.Movie, len(r.Movies))
	for i, m := range r.Movies {
		// tmdb_id lets a cheater query TMDB's public API directly for
		// the rating (higher/lower) or release year (guess_the_year);
		// the client never needs it — all scoring is server-side.
		m.TMDBID = 0
		m.IMDBRating = 0
		// The textless poster is only meaningful for blurred rounds;
		// other round types keep just the standard poster.
		m.TextlessPosterURL = ""
		if r.Type == domain.RoundGuessYear {
			m.Year = 0
		}
		cp.Movies[i] = m
	}
	return &cp
}

type blurredRoundPayload struct {
	Index     int              `json:"index"`
	Type      domain.RoundType `json:"type"`
	PosterURL string           `json:"poster_url"`
	Attempts  int              `json:"attempts"` // already spent, for mid-round reloads
}
