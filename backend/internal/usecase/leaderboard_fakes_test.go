package usecase

import (
	"context"

	"movie-trivia/internal/domain"
)

type fakeLeaderboardRepo struct {
	submitted []domain.LeaderboardEntry
}

func (f *fakeLeaderboardRepo) Submit(_ context.Context, e *domain.LeaderboardEntry) error {
	f.submitted = append(f.submitted, *e)
	return nil
}

func (f *fakeLeaderboardRepo) Top10(_ context.Context, gameDate string) ([]domain.LeaderboardEntry, error) {
	var out []domain.LeaderboardEntry
	for _, e := range f.submitted {
		if e.GameDate == gameDate {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeLeaderboardRepo) HasSubmitted(_ context.Context, gameDate, playerID string) (bool, error) {
	for _, e := range f.submitted {
		if e.GameDate == gameDate && e.PlayerID == playerID {
			return true, nil
		}
	}
	return false, nil
}

// fakeSessionRepo lives in freeplay_usecase_test.go (same package); it
// needs StatusCompleted backed by a settable map for this flow test.

func newFakeLeaderboardRepo() *fakeLeaderboardRepo {
	return &fakeLeaderboardRepo{}
}
