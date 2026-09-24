package usecase

import (
	"context"
	"strings"
	"time"

	"movie-trivia/internal/domain"
	"movie-trivia/internal/repository"
)

// LeaderboardUsecase handles the daily-only leaderboard. Submissions
// require a completed run and are one-shot per player per day.
type LeaderboardUsecase struct {
	entries  repository.LeaderboardRepo
	sessions repository.SessionRepo
	loc      *time.Location
}

func NewLeaderboardUsecase(entries repository.LeaderboardRepo, sessions repository.SessionRepo, loc *time.Location) *LeaderboardUsecase {
	return &LeaderboardUsecase{entries: entries, sessions: sessions, loc: loc}
}

func (u *LeaderboardUsecase) today() string { return time.Now().In(u.loc).Format("2006-01-02") }

func (u *LeaderboardUsecase) Submit(ctx context.Context, playerID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > domain.MaxNameLength {
		return domain.ErrInvalidName
	}
	gameDate := u.today()
	completed, err := u.sessions.StatusCompleted(ctx, playerID, gameDate)
	if err != nil {
		return err
	}
	if !completed {
		return domain.ErrNotSubmitted
	}
	submitted, err := u.entries.HasSubmitted(ctx, gameDate, playerID)
	if err != nil {
		return err
	}
	if submitted {
		return domain.ErrDuplicateSubmit
	}
	return u.entries.Submit(ctx, &domain.LeaderboardEntry{
		GameDate: gameDate, PlayerID: playerID, Name: name,
		SubmittedAt: time.Now(),
	})
}

func (u *LeaderboardUsecase) Top10(ctx context.Context) ([]domain.LeaderboardEntry, error) {
	return u.entries.Top10(ctx, u.today())
}
