package httphandler

import (
	"encoding/json"
	"testing"

	"movie-trivia/internal/domain"
)

func TestPublicRoundRedactsRating(t *testing.T) {
	orig := &domain.Round{
		Index: 1,
		Type:  domain.RoundHigherLower,
		Movies: []domain.Movie{
			{TMDBID: 1, Title: "A", IMDBRating: 8.1},
			{TMDBID: 2, Title: "B", IMDBRating: 7.2},
		},
		YearMin: 1990, YearMax: 2026,
	}
	got := publicRound(orig).(*domain.Round)

	for i, m := range got.Movies {
		if m.IMDBRating != 0 {
			t.Fatalf("movie %d rating leaked: %v", i, m.IMDBRating)
		}
		if m.TMDBID != 0 {
			t.Fatalf("movie %d tmdb_id leaked: %v", i, m.TMDBID)
		}
		if m.Title != orig.Movies[i].Title {
			t.Fatalf("movie %d title unexpectedly altered", i)
		}
	}
	if got.YearMin != 1990 || got.YearMax != 2026 || got.Index != 1 || got.Type != domain.RoundHigherLower {
		t.Fatal("round metadata unexpectedly altered")
	}
	// The stored round (with ratings) must be untouched.
	if orig.Movies[0].IMDBRating != 8.1 {
		t.Fatal("original round was mutated")
	}
}

func TestPublicRoundGuessYearRedactsAnswerYear(t *testing.T) {
	orig := &domain.Round{
		Index: 2,
		Type:  domain.RoundGuessYear,
		Movies: []domain.Movie{
			{TMDBID: 13, Title: "Forrest Gump", Year: 1994, PosterURL: "https://img/gump.jpg", IMDBRating: 8.8},
		},
		YearMin: 1957, YearMax: 2026,
	}
	b, err := json.Marshal(publicRound(orig))
	if err != nil {
		t.Fatal(err)
	}
	payload := string(b)

	for _, leak := range []string{"1994", `"year":1994`, "8.8", `"imdb_rating":8.8`, `"tmdb_id":13`} {
		if contains(payload, leak) {
			t.Fatalf("guess_the_year payload leaks %q: %s", leak, payload)
		}
	}
	if !contains(payload, "Forrest Gump") || !contains(payload, "gump.jpg") {
		t.Fatalf("guess_the_year payload missing open fields: %s", payload)
	}
	// Slider bounds must survive — the player needs them.
	if !contains(payload, "1957") || !contains(payload, "2026") {
		t.Fatalf("guess_the_year payload missing slider bounds: %s", payload)
	}
	if orig.Movies[0].Year != 1994 {
		t.Fatal("original round was mutated")
	}
}

func TestPublicRoundBlurredHidesAnswer(t *testing.T) {
	orig := &domain.Round{
		Index: 3,
		Type:  domain.RoundBlurred,
		Movies: []domain.Movie{
			{
				TMDBID:            680,
				Title:             "Pulp Fiction",
				Year:              1994,
				PosterURL:         "https://img/standard.jpg",
				TextlessPosterURL: "https://img/textless.jpg",
				IMDBRating:        8.9,
			},
		},
		YearMin: 1957, YearMax: 2026,
	}
	b, err := json.Marshal(publicRound(orig))
	if err != nil {
		t.Fatal(err)
	}
	payload := string(b)

	for _, leak := range []string{"Pulp Fiction", "1994", "tmdb_id", "standard.jpg", "imdb_rating", "textless_poster_url", "8.9"} {
		if contains(payload, leak) {
			t.Fatalf("blurred payload leaks %q: %s", leak, payload)
		}
	}
	if !contains(payload, "textless.jpg") {
		t.Fatalf("blurred payload missing textless image: %s", payload)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
