// Package usecase holds the game orchestration logic. Handlers never
// touch repositories or providers directly — all game logic, dataset
// generation, and scoring math stays on the backend.
package usecase

import (
	"context"
	"math/rand"
	"time"

	"movie-trivia/internal/domain"
)

// GameUsecase contains logic shared by both modes: the "Shuffled Bag"
// question-type distribution and round building from the master pool.
type GameUsecase struct {
	pool *PoolUsecase
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
	if len(pool) < domain.RoundsPerGame*2 {
		return nil, domain.ErrPoolEmpty
	}
	types := u.ShuffledBag()
	perm := u.rng.Perm(len(pool))
	yearMin, yearMax := poolYearBounds(pool)

	rounds := make([]domain.Round, 0, len(types))
	used := map[int]bool{}
	next := func() domain.Movie {
		for _, idx := range perm {
			if !used[idx] {
				used[idx] = true
				return pool[idx]
			}
		}
		return domain.Movie{}
	}
	for i, t := range types {
		round := domain.Round{Index: i + 1, Type: t, YearMin: yearMin, YearMax: yearMax}
		switch t {
		case domain.RoundHigherLower:
			a, b := next(), next()
			// Swap in a different movie on a rating tie (rounds become
			// ambiguous when both sides carry the same rating).
			if a.IMDBRating == b.IMDBRating {
				a = next()
			}
			round.Movies = []domain.Movie{a, b}
		default:
			round.Movies = []domain.Movie{next()}
		}
		rounds = append(rounds, round)
	}
	return rounds, nil
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
