package usecase

import (
	"context"
	"encoding/json"
	"time"

	"movie-trivia/internal/domain"
	"movie-trivia/internal/repository"
)

// GameEngine builds round variants and evaluates guesses. *GameUsecase
// is the production implementation; the interface keeps the usecase
// testable without a Redis-backed pool.
type GameEngine interface {
	BuildRounds(ctx context.Context) ([]domain.Round, error)
	EvaluateGuess(round *domain.Round, attempts int, raw json.RawMessage) (domain.AnswerOutcome, error)
}

// FreeplayUsecase serves the endless random mode. Each session carries
// its own fixed 10-round variant (persisted with the session so resume
// and later rounds serve the identical set); replaying deletes the
// previous incomplete variant and draws a fresh one.
type FreeplayUsecase struct {
	sessions repository.SessionRepo
	game     GameEngine
}

func NewFreeplayUsecase(sessions repository.SessionRepo, game GameEngine) *FreeplayUsecase {
	return &FreeplayUsecase{sessions: sessions, game: game}
}

// Start draws a new random variant and persists its session. Any
// previous incomplete variant of the player is deleted first — one
// active variant per player; completed sessions stay as history.
func (u *FreeplayUsecase) Start(ctx context.Context, playerID string) (*domain.Session, error) {
	if _, err := u.sessions.DeleteActiveByPlayer(ctx, playerID, domain.ModeFreePlay); err != nil {
		return nil, err
	}
	rounds, err := u.game.BuildRounds(ctx)
	if err != nil {
		return nil, err
	}
	s := &domain.Session{
		PlayerID: playerID, Mode: domain.ModeFreePlay,
		CurrentRound: 1, StartedAt: time.Now(),
		Rounds: rounds,
	}
	if err := u.sessions.Create(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

// ownedSession fetches the session and verifies it belongs to the
// calling player. A foreign session ID returns ErrNotFound (not a
// distinct "forbidden"), so probing other players' IDs is not possible.
func (u *FreeplayUsecase) ownedSession(ctx context.Context, playerID, sessionID string) (*domain.Session, error) {
	s, err := u.sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if s.PlayerID != playerID {
		return nil, domain.ErrNotFound
	}
	return s, nil
}

func (u *FreeplayUsecase) Round(ctx context.Context, playerID, sessionID string, n int) (*domain.Round, int, error) {
	s, err := u.ownedSession(ctx, playerID, sessionID)
	if err != nil {
		return nil, 0, err
	}
	if s.IsCompleted {
		return nil, 0, domain.ErrSessionCompleted
	}
	if n < 1 || n > len(s.Rounds) {
		return nil, 0, domain.ErrNotFound
	}
	attempts := s.Attempts
	if s.CurrentRound != n {
		attempts = 0
	}
	return &s.Rounds[n-1], attempts, nil
}

func (u *FreeplayUsecase) Answer(ctx context.Context, playerID, sessionID string, n int, raw json.RawMessage) (*domain.Session, *domain.AnswerOutcome, error) {
	s, err := u.ownedSession(ctx, playerID, sessionID)
	if err != nil {
		return nil, nil, err
	}
	if s.IsCompleted || s.CurrentRound != n {
		return nil, nil, domain.ErrSessionCompleted
	}
	if n < 1 || n > len(s.Rounds) {
		return nil, nil, domain.ErrNotFound
	}
	outcome, err := u.game.EvaluateGuess(&s.Rounds[n-1], s.Attempts, raw)
	if err != nil {
		return nil, nil, err
	}
	ApplyOutcome(s, s.Rounds[n-1].Type, outcome)
	outcome.NextRound = s.CurrentRound
	outcome.Finished = s.IsCompleted
	if err := u.sessions.Update(ctx, s, n); err != nil {
		return nil, nil, err
	}
	return s, &outcome, nil
}

func (u *FreeplayUsecase) Result(ctx context.Context, playerID, sessionID string) (any, error) {
	s, err := u.ownedSession(ctx, playerID, sessionID)
	if err != nil {
		return nil, err
	}
	if !s.IsCompleted {
		return nil, domain.ErrSessionCompleted
	}
	return map[string]any{"score": s.Score}, nil
}
