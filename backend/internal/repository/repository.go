// Package repository defines the storage interfaces shared by the
// usecase layer. Durable state lives in Postgres, ephemeral state and
// locks in Redis.
package repository

import (
	"context"
	"time"

	"movie-trivia/internal/domain"
)

// SessionRepo covers the sessions table plus the player_daily_status
// table (daily resume support).
type SessionRepo interface {
	Create(ctx context.Context, s *domain.Session) error
	Get(ctx context.Context, id string) (*domain.Session, error)
	// Update persists session progress with an optimistic guard on the
	// round that was answered: the write only lands if the stored row is
	// still on answeredRound and not completed. answeredRound is the
	// round index the caller just scored — for advancing rounds the row
	// is updated to s.CurrentRound, for blurred mid-round attempts the
	// round stays put and only Attempts change.
	Update(ctx context.Context, s *domain.Session, answeredRound int) error
	// DeleteStale garbage-collects abandoned Free Play sessions older
	// than maxAge and returns the number removed.
	DeleteStale(ctx context.Context, mode domain.GameMode, maxAge time.Duration) (int64, error)
	// DeleteActiveByPlayer removes the player's incomplete sessions of a
	// mode (used on Free Play replay: the new variant replaces the old).
	DeleteActiveByPlayer(ctx context.Context, playerID string, mode domain.GameMode) (int64, error)
	// TodaySession returns the player's session for the given game date,
	// or domain.ErrNotFound.
	TodaySession(ctx context.Context, playerID, gameDate string) (*domain.Session, error)
	// UpsertStatus records daily progress for a player_id.
	UpsertStatus(ctx context.Context, playerID, gameDate string, completed bool) error
	// StatusCompleted reports whether the player completed today's run.
	StatusCompleted(ctx context.Context, playerID, gameDate string) (bool, error)
}

// DailyGameRepo covers the daily_games table (UNIQUE(game_date)).
type DailyGameRepo interface {
	Upsert(ctx context.Context, gameDate string, rounds []domain.Round) error
	Get(ctx context.Context, gameDate string) ([]domain.Round, error)
}

// LeaderboardRepo covers the leaderboard_entries table
// (UNIQUE(game_date, player_id) — one submission per player per day).
type LeaderboardRepo interface {
	Submit(ctx context.Context, e *domain.LeaderboardEntry) error
	Top10(ctx context.Context, gameDate string) ([]domain.LeaderboardEntry, error)
	HasSubmitted(ctx context.Context, gameDate, playerID string) (bool, error)
}

// PoolRepo is the durable mirror of the master pool in Postgres, used
// to rebuild the Redis cache after a cache wipe.
type PoolRepo interface {
	Save(ctx context.Context, movies []domain.Movie) error
	List(ctx context.Context) ([]domain.Movie, error)
}
