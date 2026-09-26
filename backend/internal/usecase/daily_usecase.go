package usecase

import (
	"context"
	"encoding/json"
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
	if err == domain.ErrNotFound {
		// The daily chain hasn't generated today's set yet (e.g. a
		// player arrives right after midnight). Tell the client to
		// retry shortly instead of failing with a generic 404.
		return nil, domain.ErrGamePreparing
	}
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

// Round returns round n of today's fixed set, plus the attempts already
// spent on it (so a mid-round reload restores the attempt UI).
func (u *DailyUsecase) Round(ctx context.Context, playerID string, n int) (*domain.Round, int, error) {
	s, err := u.sessions.TodaySession(ctx, playerID, u.today())
	if err != nil {
		return nil, 0, err
	}
	if s.IsCompleted {
		return nil, 0, domain.ErrSessionCompleted
	}
	rounds, err := u.dailyGames.Get(ctx, u.today())
	if err != nil {
		return nil, 0, err
	}
	if n < 1 || n > len(rounds) {
		return nil, 0, domain.ErrNotFound
	}
	attempts := s.Attempts
	if s.CurrentRound != n {
		attempts = 0 // already-answered rounds show a clean slate
	}
	return &rounds[n-1], attempts, nil
}

// Answer evaluates a guess against today's fixed set, scores it,
// advances the session, and returns the result (correct/incorrect,
// reveal data, points earned). Completing round 10 locks the run.
func (u *DailyUsecase) Answer(ctx context.Context, playerID string, n int, raw json.RawMessage) (*domain.Session, *domain.AnswerOutcome, error) {
	s, err := u.sessions.TodaySession(ctx, playerID, u.today())
	if err != nil {
		return nil, nil, err
	}
	if s.IsCompleted || s.CurrentRound != n {
		return nil, nil, domain.ErrSessionCompleted
	}
	rounds, err := u.dailyGames.Get(ctx, u.today())
	if err != nil {
		return nil, nil, err
	}
	if n < 1 || n > len(rounds) {
		return nil, nil, domain.ErrNotFound
	}
	outcome, err := u.game.EvaluateGuess(&rounds[n-1], s.Attempts, raw)
	if err != nil {
		return nil, nil, err
	}
	ApplyOutcome(s, rounds[n-1].Type, outcome)
	outcome.NextRound = s.CurrentRound
	outcome.Finished = s.IsCompleted
	if err := u.sessions.Update(ctx, s, n); err != nil {
		return nil, nil, err
	}
	if s.IsCompleted {
		if err := u.sessions.UpsertStatus(ctx, playerID, u.today(), true); err != nil {
			return nil, nil, err
		}
	}
	return s, &outcome, nil
}

// Result returns the final score for today's run.
func (u *DailyUsecase) Result(ctx context.Context, playerID string) (any, error) {
	s, err := u.sessions.TodaySession(ctx, playerID, u.today())
	if err != nil {
		return nil, err
	}
	if !s.IsCompleted {
		return nil, domain.ErrNotSubmitted
	}
	return map[string]any{"score": s.Score}, nil
}

// Status reports today's completion state for the player and whether
// they already submitted a leaderboard entry today.
func (u *DailyUsecase) Status(ctx context.Context, playerID string) (completed, submitted bool, err error) {
	completed, err = u.sessions.StatusCompleted(ctx, playerID, u.today())
	if err != nil {
		return false, false, err
	}
	if !completed {
		return false, false, nil
	}
	submitted, err = u.leaderboard.HasSubmitted(ctx, u.today(), playerID)
	if err != nil {
		return false, false, err
	}
	return completed, submitted, nil
}

// Top10 returns today's leaderboard entries.
func (u *DailyUsecase) Top10(ctx context.Context) ([]domain.LeaderboardEntry, error) {
	return u.leaderboard.Top10(ctx, u.today())
}
