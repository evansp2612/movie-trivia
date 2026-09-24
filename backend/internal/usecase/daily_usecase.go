package usecase

import (
	"context"
	"time"

	"movie-trivia/internal/domain"
	"movie-trivia/internal/repository"
)

// DailyUsecase orchestrates the Game of the Day. The run is globally
// fixed per calendar day and resumable until completed.
type DailyUsecase struct {
	sessions    repository.SessionRepo
	dailyGames  repository.DailyGameRepo
	leaderboard repository.LeaderboardRepo
	game        *GameUsecase
	loc         *time.Location
}

func NewDailyUsecase(sessions repository.SessionRepo, dailyGames repository.DailyGameRepo,
	leaderboard repository.LeaderboardRepo, game *GameUsecase, loc *time.Location) *DailyUsecase {
	return &DailyUsecase{sessions: sessions, dailyGames: dailyGames, leaderboard: leaderboard, game: game, loc: loc}
}

func (u *DailyUsecase) today() string { return time.Now().In(u.loc).Format("2006-01-02") }

// Start returns today's session, resuming an incomplete run; a
// completed run yields ErrAlreadyCompleted (mapped to 409).
func (u *DailyUsecase) Start(ctx context.Context, playerID string) (*domain.Session, error) {
	gameDate := u.today()
	completed, err := u.sessions.StatusCompleted(ctx, playerID, gameDate)
	if err != nil {
		return nil, err
	}
	if completed {
		return nil, domain.ErrAlreadyCompleted
	}
	s, err := u.sessions.TodaySession(ctx, playerID, gameDate)
	if err == nil && !s.IsCompleted {
		return s, nil // resume
	}
	if err != nil && err != domain.ErrNotFound {
		return nil, err
	}
	rounds, err := u.dailyGames.Get(ctx, gameDate)
	if err != nil {
		return nil, err
	}
	_ = rounds // the fixed set lives on daily_games; served per index
	s = &domain.Session{
		PlayerID: playerID, Mode: domain.ModeDaily,
		CurrentRound: 1, StartedAt: time.Now(), GameDate: gameDate,
	}
	if err := u.sessions.Create(ctx, s); err != nil {
		return nil, err
	}
	return s, u.sessions.UpsertStatus(ctx, playerID, gameDate, false)
}

// Round returns round n of today's fixed set.
func (u *DailyUsecase) Round(ctx context.Context, playerID string, n int) (*domain.Round, error) {
	s, err := u.sessions.TodaySession(ctx, playerID, u.today())
	if err != nil {
		return nil, err
	}
	if s.IsCompleted {
		return nil, domain.ErrSessionCompleted
	}
	rounds, err := u.dailyGames.Get(ctx, u.today())
	if err != nil {
		return nil, err
	}
	if n < 1 || n > len(rounds) {
		return nil, domain.ErrNotFound
	}
	return &rounds[n-1], nil
}

// Answer scores a guess, advances the session, and returns the round
// result (correct/incorrect, actual answers, points earned).
func (u *DailyUsecase) Answer(ctx context.Context, playerID string, n int, guess any) (*domain.Session, error) {
	s, err := u.sessions.TodaySession(ctx, playerID, u.today())
	if err != nil {
		return nil, err
	}
	if s.IsCompleted || s.CurrentRound != n {
		return nil, domain.ErrSessionCompleted
	}
	// TODO: validate guess per round type, compute points via
	// domain.ScoreForRound, persist, and flip is_completed + status on
	// the 10th round.
	_ = guess
	return s, domain.ErrNotImplemented
}

// Result returns the final score and per-type breakdown for today.
func (u *DailyUsecase) Result(ctx context.Context, playerID string) (any, error) {
	// TODO: aggregate per-question-type breakdown.
	_ = ctx
	_ = playerID
	return nil, domain.ErrNotImplemented
}

// Status reports today's completion state for the player.
func (u *DailyUsecase) Status(ctx context.Context, playerID string) (bool, error) {
	return u.sessions.StatusCompleted(ctx, playerID, u.today())
}

// SubmitLeaderboard records the one-shot name submission for today.
func (u *DailyUsecase) SubmitLeaderboard(ctx context.Context, playerID, name string) error {
	// TODO: validate name length (<= domain.MaxNameLength), require
	// completed run, reject duplicates (UNIQUE constraint).
	_ = ctx
	_, _ = playerID, name
	return domain.ErrNotImplemented
}

// Top10 returns today's leaderboard entries.
func (u *DailyUsecase) Top10(ctx context.Context) ([]domain.LeaderboardEntry, error) {
	return u.leaderboard.Top10(ctx, u.today())
}
