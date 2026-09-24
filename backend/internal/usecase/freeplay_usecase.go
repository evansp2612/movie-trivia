package usecase

import (
	"context"
	"time"

	"movie-trivia/internal/domain"
	"movie-trivia/internal/repository"
)

// FreeplayUsecase serves the endless random mode. Sessions are drawn
// from the unified master pool; abandoned sessions are garbage-collected
// by a background timer.
type FreeplayUsecase struct {
	sessions repository.SessionRepo
	game     *GameUsecase
}

func NewFreeplayUsecase(sessions repository.SessionRepo, game *GameUsecase) *FreeplayUsecase {
	return &FreeplayUsecase{sessions: sessions, game: game}
}

// Start builds a random 10-round run and persists its session.
func (u *FreeplayUsecase) Start(ctx context.Context, playerID string) (*domain.Session, error) {
	rounds, err := u.game.BuildRounds(ctx)
	if err != nil {
		return nil, err
	}
	_ = rounds // TODO: persist per-session rounds (or derive deterministically from a session seed)
	s := &domain.Session{
		PlayerID: playerID, Mode: domain.ModeFreePlay,
		CurrentRound: 1, StartedAt: time.Now(),
	}
	if err := u.sessions.Create(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

func (u *FreeplayUsecase) Round(ctx context.Context, sessionID string, n int) (*domain.Round, error) {
	s, err := u.sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if s.IsCompleted {
		return nil, domain.ErrSessionCompleted
	}
	// TODO: rebuild the session's round set and return round n.
	_ = n
	return nil, domain.ErrNotImplemented
}

func (u *FreeplayUsecase) Answer(ctx context.Context, sessionID string, n int, guess any) (*domain.Session, error) {
	s, err := u.sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if s.IsCompleted || s.CurrentRound != n {
		return nil, domain.ErrSessionCompleted
	}
	// TODO: score via domain.ScoreForRound, persist, complete at round 10.
	_ = guess
	return s, domain.ErrNotImplemented
}

func (u *FreeplayUsecase) Result(ctx context.Context, sessionID string) (any, error) {
	// TODO: final score + breakdown. Free Play has no leaderboard.
	_ = ctx
	_ = sessionID
	return nil, domain.ErrNotImplemented
}
