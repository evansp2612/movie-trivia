package usecase

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"os"
	"testing"
	"time"

	"movie-trivia/internal/domain"
)

// ---- fakes ----

type fakePool struct {
	movies []domain.Movie
}

func (f *fakePool) Candidates(context.Context) ([]domain.Movie, error) {
	return f.movies, nil
}

func mixedPool() []domain.Movie {
	// 40 fully valid; 10 without rating; 10 without textless; 10 without year.
	var movies []domain.Movie
	for i := 0; i < 40; i++ {
		movies = append(movies, domain.Movie{
			TMDBID: 100 + i, Title: string(rune('A'+i%26)) + string(rune('a'+i)), Year: 1970 + i,
			PosterURL: "p.jpg", TextlessPosterURL: "t.jpg", IMDBRating: 6.0 + float64(i%40)/10,
		})
	}
	for i := 0; i < 10; i++ { // no rating
		movies = append(movies, domain.Movie{TMDBID: 200 + i, Title: "NoRating", Year: 2000 + i, PosterURL: "p.jpg", TextlessPosterURL: "t.jpg"})
	}
	for i := 0; i < 10; i++ { // no textless
		movies = append(movies, domain.Movie{TMDBID: 300 + i, Title: "NoTextless", Year: 2000 + i, PosterURL: "p.jpg", IMDBRating: 7.1})
	}
	for i := 0; i < 10; i++ { // no year
		movies = append(movies, domain.Movie{TMDBID: 400 + i, Title: "NoYear", PosterURL: "p.jpg", TextlessPosterURL: "t.jpg", IMDBRating: 7.2})
	}
	return movies
}

func newBuildTestGame(pool []domain.Movie) *GameUsecase {
	return &GameUsecase{pool: &fakePool{movies: pool}, rng: rand.New(rand.NewSource(1))}
}

// ---- tests ----

func TestBuildRoundsFiltersByRoundType(t *testing.T) {
	u := newBuildTestGame(mixedPool())
	rounds, err := u.BuildRounds(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rounds) != domain.RoundsPerGame {
		t.Fatalf("got %d rounds", len(rounds))
	}

	seen := map[int]bool{}
	for _, r := range rounds {
		if len(r.Movies) == 0 {
			t.Fatalf("round %d has no movies", r.Index)
		}
		for _, m := range r.Movies {
			if seen[m.TMDBID] {
				t.Fatalf("movie %d used twice (round %d)", m.TMDBID, r.Index)
			}
			seen[m.TMDBID] = true
			switch r.Type {
			case domain.RoundHigherLower:
				if m.IMDBRating <= 0 {
					t.Fatalf("higher/lower round %d got unrated movie %d", r.Index, m.TMDBID)
				}
			case domain.RoundBlurred:
				if m.TextlessPosterURL == "" {
					t.Fatalf("blurred round %d got movie %d without textless poster", r.Index, m.TMDBID)
				}
				if m.Title != "" && m.TMDBID >= 300 && m.TMDBID < 400 {
					t.Fatalf("blurred round got textless-less partition movie %d", m.TMDBID)
				}
			case domain.RoundGuessYear:
				if m.Year <= 0 {
					t.Fatalf("year round %d got movie %d without year", r.Index, m.TMDBID)
				}
				if m.Year >= time.Now().Year() {
					t.Fatalf("year round %d got current-year movie %d (%d)", r.Index, m.TMDBID, m.Year)
				}
			}
		}
	}
}

func TestBuildRoundsUndersizedPartition(t *testing.T) {
	// Silence the per-attempt diagnostic logs; the assertion is what matters.
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	// Pool where too few movies have ratings to fill the higher/lower rounds.
	pool := []domain.Movie{
		{TMDBID: 1, Title: "A", Year: 2000, TextlessPosterURL: "t.jpg", IMDBRating: 7.0},
		{TMDBID: 2, Title: "B", Year: 2001, TextlessPosterURL: "t.jpg", IMDBRating: 7.1},
		{TMDBID: 3, Title: "C", Year: 2002, TextlessPosterURL: "t.jpg"}, // no rating
		{TMDBID: 4, Title: "D", Year: 2003, TextlessPosterURL: "t.jpg"}, // no rating
		{TMDBID: 5, Title: "E", Year: 2004, TextlessPosterURL: "t.jpg"}, // no rating
	}
	for i := 0; i < 100; i++ {
		if _, err := newBuildTestGame(pool).BuildRounds(context.Background()); err != domain.ErrPoolEmpty {
			t.Fatalf("want ErrPoolEmpty, got %v", err)
		}
	}
}

func TestBuildRoundsBlurredNeedsTextless(t *testing.T) {
	// Enough ratable/dated movies but no textless at all.
	var pool []domain.Movie
	for i := 0; i < 30; i++ {
		pool = append(pool, domain.Movie{TMDBID: 500 + i, Title: "X", Year: 1990 + i, PosterURL: "p.jpg", IMDBRating: 7.0})
	}
	if _, err := newBuildTestGame(pool).BuildRounds(context.Background()); err != domain.ErrPoolEmpty {
		t.Fatalf("want ErrPoolEmpty, got %v", err)
	}
}

func TestShuffledBagDistribution(t *testing.T) {
	u := &GameUsecase{rng: rand.New(rand.NewSource(1))}
	counts := map[domain.RoundType]int{}
	for _, rt := range u.ShuffledBag() {
		counts[rt]++
	}
	if counts[domain.RoundHigherLower] != 3 ||
		counts[domain.RoundBlurred] != 3 ||
		counts[domain.RoundGuessYear] != 4 {
		t.Fatalf("expected 3-3-4 distribution, got %v", counts)
	}
}

func TestShuffledBagNoTripleRun(t *testing.T) {
	u := &GameUsecase{rng: rand.New(rand.NewSource(42))}
	for i := 0; i < 500; i++ {
		bag := u.ShuffledBag()
		for j := 2; j < len(bag); j++ {
			if bag[j] == bag[j-1] && bag[j] == bag[j-2] {
				t.Fatalf("type %q repeats 3 times in a row at %d: %v", bag[j], j, bag)
			}
		}
	}
}

func TestScoreForRound(t *testing.T) {
	cases := []struct {
		name    string
		rt      domain.RoundType
		correct bool
		attempt int
		yearGap int
		want    int
	}{
		{"higher lower correct", domain.RoundHigherLower, true, 0, 0, 10},
		{"higher lower wrong", domain.RoundHigherLower, false, 0, 0, 0},
		{"blurred attempt 1", domain.RoundBlurred, true, 1, 0, 10},
		{"blurred attempt 3", domain.RoundBlurred, true, 3, 0, 6},
		{"blurred attempt 5", domain.RoundBlurred, true, 5, 0, 2},
		{"blurred failed", domain.RoundBlurred, false, 5, 0, 0},
		{"year exact", domain.RoundGuessYear, true, 0, 0, 10},
		{"year off by 3", domain.RoundGuessYear, true, 0, 3, 7},
		{"year off by 10 floors", domain.RoundGuessYear, true, 0, 10, 0},
		{"year off by 15 floors", domain.RoundGuessYear, true, 0, 15, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.ScoreForRound(tc.rt, tc.correct, tc.attempt, tc.yearGap)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

// ---- EvaluateGuess / ApplyOutcome ----

func hlRound(a, b float64) *domain.Round {
	return &domain.Round{
		Index: 1, Type: domain.RoundHigherLower,
		Movies: []domain.Movie{
			{TMDBID: 1, Title: "A", IMDBRating: a},
			{TMDBID: 2, Title: "B", IMDBRating: b},
		},
	}
}

func TestEvaluateGuessHigherLower(t *testing.T) {
	u := &GameUsecase{}
	raw := func(s string) json.RawMessage { return json.RawMessage(s) }

	out, err := u.EvaluateGuess(hlRound(8.5, 7.0), 0, raw(`{"choice":0}`))
	if err != nil || !out.Correct || out.Points != 10 {
		t.Fatalf("choosing higher card: %+v, %v", out, err)
	}
	if ratings := out.Actual.(map[string]any)["ratings"].([]float64); ratings[0] != 8.5 || ratings[1] != 7.0 {
		t.Fatalf("reveal ratings wrong: %v", ratings)
	}

	out, err = u.EvaluateGuess(hlRound(8.5, 7.0), 0, raw(`{"choice":1}`))
	if err != nil || out.Correct || out.Points != 0 {
		t.Fatalf("choosing lower card: %+v, %v", out, err)
	}

	// Tie is never correct (BuildRounds avoids ties, but be safe).
	out, _ = u.EvaluateGuess(hlRound(7.0, 7.0), 0, raw(`{"choice":0}`))
	if out.Correct {
		t.Fatal("tie must not be correct")
	}

	if _, err := u.EvaluateGuess(hlRound(8.5, 7.0), 0, raw(`{"choice":5}`)); err != domain.ErrInvalidGuess {
		t.Fatalf("choice 5: want ErrInvalidGuess, got %v", err)
	}
	if _, err := u.EvaluateGuess(hlRound(8.5, 7.0), 0, raw(`{}`)); err != domain.ErrInvalidGuess {
		t.Fatalf("missing choice: want ErrInvalidGuess, got %v", err)
	}
}

func TestEvaluateGuessBlurred(t *testing.T) {
	u := &GameUsecase{}
	round := &domain.Round{Index: 2, Type: domain.RoundBlurred, Movies: []domain.Movie{{Title: "Pulp Fiction"}}}
	raw := func(s string) json.RawMessage { return json.RawMessage(s) }

	// Correct on attempt 1 → 10 pts.
	out, err := u.EvaluateGuess(round, 0, raw(`{"title":"  pulp fiction  "}`))
	if err != nil || !out.Correct || out.Points != 10 || out.Attempts != 1 {
		t.Fatalf("correct blurred: %+v, %v", out, err)
	}

	// Misses score 0 but burn attempts.
	for i := 1; i <= 4; i++ {
		out, err := u.EvaluateGuess(round, i, raw(`{"title":"Wrong"}`))
		if err != nil || out.Correct || out.Points != 0 || out.Attempts != i+1 {
			t.Fatalf("miss after %d attempts: %+v, %v", i, out, err)
		}
	}

	// 5th miss: 0 points, attempts capped at 5.
	out, _ = u.EvaluateGuess(round, 4, raw(`{"title":"Wrong"}`))
	if out.Correct || out.Points != 0 || out.Attempts != 5 {
		t.Fatalf("5th miss: %+v", out)
	}

	// Correcting on a later attempt earns the ladder value: attempt 3 → 6.
	out, _ = u.EvaluateGuess(round, 2, raw(`{"title":"Pulp Fiction"}`))
	if !out.Correct || out.Points != 6 || out.Attempts != 3 {
		t.Fatalf("correct on attempt 3: %+v", out)
	}
}

func TestEvaluateGuessYear(t *testing.T) {
	u := &GameUsecase{}
	round := &domain.Round{Index: 3, Type: domain.RoundGuessYear, Movies: []domain.Movie{{Title: "A", Year: 1994}}}
	raw := func(s string) json.RawMessage { return json.RawMessage(s) }

	out, err := u.EvaluateGuess(round, 0, raw(`{"year":1994}`))
	if err != nil || !out.Correct || out.Points != 10 {
		t.Fatalf("exact year: %+v, %v", out, err)
	}
	if y := out.Actual.(map[string]any)["year"].(int); y != 1994 {
		t.Fatalf("reveal year: %v", y)
	}

	out, _ = u.EvaluateGuess(round, 0, raw(`{"year":1997}`))
	if out.Correct || out.Points != 7 {
		t.Fatalf("3 off: %+v", out)
	}
	out, _ = u.EvaluateGuess(round, 0, raw(`{"year":1984}`))
	if out.Correct || out.Points != 0 {
		t.Fatalf("10 off floors: %+v", out)
	}
	if _, err := u.EvaluateGuess(round, 0, raw(`{"year":"x"}`)); err != domain.ErrInvalidGuess {
		t.Fatalf("bad year: want ErrInvalidGuess, got %v", err)
	}
}

func TestApplyOutcomeProgression(t *testing.T) {
	s := &domain.Session{CurrentRound: 1}

	// Correct answer advances the round.
	ApplyOutcome(s, domain.RoundHigherLower, domain.AnswerOutcome{Correct: true, Points: 10})
	if s.Score != 10 || s.CurrentRound != 2 || s.Attempts != 0 {
		t.Fatalf("after correct: %+v", s)
	}

	// Blurred mid-attempt does not advance but stores attempts.
	ApplyOutcome(s, domain.RoundBlurred, domain.AnswerOutcome{Correct: false, Points: 0, Attempts: 1})
	if s.CurrentRound != 2 || s.Attempts != 1 {
		t.Fatalf("after blurred miss: %+v", s)
	}

	// 10 correct answers from round 2 complete the game.
	for i := 2; i <= 10; i++ {
		ApplyOutcome(s, domain.RoundGuessYear, domain.AnswerOutcome{Correct: true, Points: 10})
	}
	if !s.IsCompleted || s.CurrentRound != 11 || s.Score != 100 {
		t.Fatalf("after 10 rounds: %+v", s)
	}
}

func TestApplyOutcomeBlurredFifthMissAdvances(t *testing.T) {
	s := &domain.Session{CurrentRound: 3, Attempts: 4}
	ApplyOutcome(s, domain.RoundBlurred, domain.AnswerOutcome{Correct: false, Points: 0, Attempts: 5})
	if s.CurrentRound != 4 || s.Attempts != 0 || s.Score != 0 {
		t.Fatalf("blurred failed round should advance with 0 pts: %+v", s)
	}
}

// Regression: a WRONG higher/lower or year answer must still advance the
// round (one guess per round, per the PRD). Before this fix only correct
// answers advanced, stranding the session and 409ing every later submit.
func TestApplyOutcomeWrongAnswerAdvances(t *testing.T) {
	s := &domain.Session{CurrentRound: 2}
	ApplyOutcome(s, domain.RoundHigherLower, domain.AnswerOutcome{Correct: false, Points: 0})
	if s.CurrentRound != 3 {
		t.Fatalf("wrong HL answer must advance: %+v", s)
	}

	s = &domain.Session{CurrentRound: 2}
	ApplyOutcome(s, domain.RoundGuessYear, domain.AnswerOutcome{Correct: false, Points: 5})
	if s.CurrentRound != 3 || s.Score != 5 {
		t.Fatalf("wrong year answer must advance with points: %+v", s)
	}

	// Blurred misses below 5 must NOT advance (attempt ladder).
	s = &domain.Session{CurrentRound: 2, Attempts: 1}
	ApplyOutcome(s, domain.RoundBlurred, domain.AnswerOutcome{Correct: false, Points: 0, Attempts: 2})
	if s.CurrentRound != 2 || s.Attempts != 2 {
		t.Fatalf("blurred miss 2 must stay on the round: %+v", s)
	}
}
