package usecase

import (
	"math/rand"
	"testing"

	"movie-trivia/internal/domain"
)

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
