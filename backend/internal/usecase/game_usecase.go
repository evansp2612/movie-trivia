// Package usecase holds the game orchestration logic. Handlers never
// touch repositories or providers directly — all game logic, dataset
// generation, and scoring math stays on the backend.
package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"movie-trivia/internal/domain"
)

// poolSource supplies the candidate pool; *PoolUsecase is the
// production implementation, the interface keeps BuildRounds testable.
type poolSource interface {
	Candidates(ctx context.Context) ([]domain.Movie, error)
}

// GameUsecase contains logic shared by both modes: the "Shuffled Bag"
// question-type distribution and round building from the master pool.
type GameUsecase struct {
	pool poolSource
	rng  *rand.Rand
}

func NewGameUsecase(pool *PoolUsecase) *GameUsecase {
	return &GameUsecase{pool: pool, rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// ShuffledBag returns 10 question types in a strict 3-3-4 distribution
// (the type appearing 4 times is chosen at random), shuffled so that no
// question type repeats more than twice in a row.
func (u *GameUsecase) ShuffledBag() []domain.RoundType {
	bag := []domain.RoundType{
		domain.RoundHigherLower, domain.RoundHigherLower, domain.RoundHigherLower,
		domain.RoundBlurred, domain.RoundBlurred, domain.RoundBlurred,
		domain.RoundGuessYear, domain.RoundGuessYear, domain.RoundGuessYear,
	}
	fourth := []domain.RoundType{domain.RoundHigherLower, domain.RoundBlurred, domain.RoundGuessYear}[u.rng.Intn(3)]
	bag = append(bag, fourth)

	u.rng.Shuffle(len(bag), func(i, j int) { bag[i], bag[j] = bag[j], bag[i] })

	// Re-validate: cap any run of the same type at 2. A run at i is
	// fixed by swapping with a distant element; the swap is kept only
	// if it introduces no new triple in the windows around either
	// swapped position (pre-existing triples elsewhere are handled by
	// later iterations).
	tripleInWindow := func(bag []domain.RoundType, lo, hi int) bool {
		if lo < 0 {
			lo = 0
		}
		for j := lo + 2; j <= hi && j < len(bag); j++ {
			if bag[j] == bag[j-1] && bag[j] == bag[j-2] {
				return true
			}
		}
		return false
	}
	for i := 2; i < len(bag); i++ {
		if bag[i] != bag[i-1] || bag[i] != bag[i-2] {
			continue
		}
		for j := 0; j < len(bag); j++ {
			if j >= i-2 && j <= i+2 {
				continue
			}
			bag[i], bag[j] = bag[j], bag[i]
			if !tripleInWindow(bag, i-2, i+2) && !tripleInWindow(bag, j-2, j+2) {
				break
			}
			bag[i], bag[j] = bag[j], bag[i]
		}
	}
	return bag
}

// BuildRounds assembles a full 10-round game from the current master
// pool. Each movie appears at most once per run.
func (u *GameUsecase) BuildRounds(ctx context.Context) ([]domain.Round, error) {
	pool, err := u.pool.Candidates(ctx)
	if err != nil {
		return nil, err
	}
	types := u.ShuffledBag()
	yearMin, yearMax := poolYearBounds(pool)

	// Partition the pool by round-suitability. The pool is a raw
	// candidate store; rounds may only use movies that satisfy their
	// type: higher/lower needs a rating, blurred needs a textless
	// poster, guess-the-year needs a real release year — current-year
	// movies are excluded from year rounds (their date is the topic of
	// the news, not trivia, and the pool is full of them).
	ratable, textless, dated := partitionPool(pool, time.Now().Year())
	hlCount, blCount, yrCount := countTypes(types)
	if len(ratable) < hlCount*2 {
		log.Printf("BuildRounds: pool has %d ratable movies, need %d", len(ratable), hlCount*2)
		return nil, domain.ErrPoolEmpty
	}
	if len(textless) < blCount {
		log.Printf("BuildRounds: pool has %d textless posters, need %d", len(textless), blCount)
		return nil, domain.ErrPoolEmpty
	}
	if len(dated) < yrCount {
		log.Printf("BuildRounds: pool has %d dated movies, need %d", len(dated), yrCount)
		return nil, domain.ErrPoolEmpty
	}

	// next draws a movie from one partition; the shared used-set keeps
	// every movie at most once per game across all rounds.
	used := map[int]bool{}
	drawer := func(from []domain.Movie) domain.Movie {
		for _, idx := range u.rng.Perm(len(from)) {
			m := from[idx]
			if !used[m.TMDBID] {
				used[m.TMDBID] = true
				return m
			}
		}
		return domain.Movie{}
	}

	rounds := make([]domain.Round, 0, len(types))
	for i, t := range types {
		round := domain.Round{Index: i + 1, Type: t, YearMin: yearMin, YearMax: yearMax}
		switch t {
		case domain.RoundHigherLower:
			a, b := drawer(ratable), drawer(ratable)
			// Swap in a different movie on a rating tie (rounds become
			// ambiguous when both sides carry the same rating).
			if a.IMDBRating == b.IMDBRating {
				if again := drawer(ratable); again.TMDBID != 0 {
					a = again
				}
			}
			round.Movies = []domain.Movie{a, b}
		case domain.RoundBlurred:
			round.Movies = []domain.Movie{drawer(textless)}
		default:
			round.Movies = []domain.Movie{drawer(dated)}
		}
		rounds = append(rounds, round)
	}
	return rounds, nil
}

func partitionPool(pool []domain.Movie, currentYear int) (ratable, textless, dated []domain.Movie) {
	ratable = make([]domain.Movie, 0, len(pool))
	textless = make([]domain.Movie, 0, len(pool))
	dated = make([]domain.Movie, 0, len(pool))
	for _, m := range pool {
		if m.IMDBRating > 0 {
			ratable = append(ratable, m)
		}
		if m.TextlessPosterURL != "" {
			textless = append(textless, m)
		}
		// Year rounds: a real past release year — "this year" movies are
		// excluded so the answer is settled trivia, not a 2026 release.
		if m.Year > 0 && m.Year < currentYear {
			dated = append(dated, m)
		}
	}
	return ratable, textless, dated
}

func countTypes(types []domain.RoundType) (hl, bl, yr int) {
	for _, t := range types {
		switch t {
		case domain.RoundHigherLower:
			hl++
		case domain.RoundBlurred:
			bl++
		default:
			yr++
		}
	}
	return hl, bl, yr
}

func poolYearBounds(pool []domain.Movie) (min, max int) {
	for i, m := range pool {
		if i == 0 || m.Year < min {
			min = m.Year
		}
		if i == 0 || m.Year > max {
			max = m.Year
		}
	}
	if min > max {
		min, max = max, min
	}
	return min, max
}

// EvaluateGuess parses a raw guess for the round's type, compares it to
// the answer, and scores it. It never mutates the session — callers
// apply the outcome via ApplyOutcome. Returns ErrInvalidGuess when the
// guess body doesn't match the round type's expected shape.
func (u *GameUsecase) EvaluateGuess(round *domain.Round, attempts int, raw json.RawMessage) (domain.AnswerOutcome, error) {
	switch round.Type {
	case domain.RoundHigherLower:
		if len(round.Movies) != 2 {
			return domain.AnswerOutcome{}, fmt.Errorf("higher/lower round %d malformed", round.Index)
		}
		var g struct {
			Choice *int `json:"choice"`
		}
		if err := json.Unmarshal(raw, &g); err != nil || g.Choice == nil || (*g.Choice != 0 && *g.Choice != 1) {
			return domain.AnswerOutcome{}, domain.ErrInvalidGuess
		}
		correct := round.Movies[*g.Choice].IMDBRating > round.Movies[1-*g.Choice].IMDBRating
		points, _ := domain.ScoreForRound(round.Type, correct, 0, 0)
		return domain.AnswerOutcome{
			Correct: correct,
			Points:  points,
			Actual:  map[string]any{"ratings": []float64{round.Movies[0].IMDBRating, round.Movies[1].IMDBRating}},
		}, nil

	case domain.RoundBlurred:
		if len(round.Movies) != 1 {
			return domain.AnswerOutcome{}, fmt.Errorf("blurred round %d malformed", round.Index)
		}
		var g struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal(raw, &g); err != nil {
			return domain.AnswerOutcome{}, domain.ErrInvalidGuess
		}
		attemptsUsed := attempts + 1
		if attemptsUsed > domain.BlurredMaxTries {
			attemptsUsed = domain.BlurredMaxTries
		}
		correct := strings.EqualFold(strings.TrimSpace(g.Title), strings.TrimSpace(round.Movies[0].Title))
		points, _ := domain.ScoreForRound(round.Type, correct, attemptsUsed, 0)
		return domain.AnswerOutcome{
			Correct:  correct,
			Points:   points,
			Attempts: attemptsUsed,
			Actual:   map[string]any{"title": round.Movies[0].Title},
		}, nil

	case domain.RoundGuessYear:
		if len(round.Movies) != 1 {
			return domain.AnswerOutcome{}, fmt.Errorf("year round %d malformed", round.Index)
		}
		var g struct {
			Year *int `json:"year"`
		}
		if err := json.Unmarshal(raw, &g); err != nil || g.Year == nil {
			return domain.AnswerOutcome{}, domain.ErrInvalidGuess
		}
		diff := *g.Year - round.Movies[0].Year
		if diff < 0 {
			diff = -diff
		}
		correct := diff == 0
		points, _ := domain.ScoreForRound(round.Type, correct, 0, diff)
		return domain.AnswerOutcome{
			Correct: correct,
			Points:  points,
			Actual:  map[string]any{"year": round.Movies[0].Year},
		}, nil

	default:
		return domain.AnswerOutcome{}, fmt.Errorf("unknown round type %q", round.Type)
	}
}

// ApplyOutcome mutates the session with an evaluated guess: points are
// added to the score and the round ends after ONE answer — except the
// blurred poster, which allows up to 5 attempts and only ends on a
// correct guess or the 5th miss (wrong higher/lower and year guesses
// still advance: one guess per round, then feedback).
// Completing round 10 flips IsCompleted.
func ApplyOutcome(s *domain.Session, roundType domain.RoundType, outcome domain.AnswerOutcome) {
	s.Score += outcome.Points
	roundOver := outcome.Correct ||
		roundType != domain.RoundBlurred ||
		outcome.Attempts >= domain.BlurredMaxTries
	if roundOver {
		s.CurrentRound++
		s.Attempts = 0
	} else {
		s.Attempts = outcome.Attempts
	}
	if s.CurrentRound > domain.RoundsPerGame {
		s.IsCompleted = true
	}
}
