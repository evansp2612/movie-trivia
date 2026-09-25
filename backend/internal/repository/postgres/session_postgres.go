package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"movie-trivia/internal/domain"
)

type SessionRepo struct{ db *sql.DB }

func NewSessionRepo(db *sql.DB) *SessionRepo { return &SessionRepo{db: db} }

func (r *SessionRepo) Create(ctx context.Context, s *domain.Session) error {
	var rounds any
	if len(s.Rounds) > 0 {
		b, err := json.Marshal(s.Rounds)
		if err != nil {
			return err
		}
		rounds = b
	}
	return r.db.QueryRowContext(ctx,
		`INSERT INTO sessions (player_id, mode, current_round, score, is_completed, attempts, started_at, game_date, rounds)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		s.PlayerID, s.Mode, s.CurrentRound, s.Score, s.IsCompleted, s.Attempts, s.StartedAt, nullable(s.GameDate), rounds,
	).Scan(&s.ID)
}

func (r *SessionRepo) Get(ctx context.Context, id string) (*domain.Session, error) {
	s := &domain.Session{}
	var gameDate sql.NullString
	var rounds []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT id, player_id, mode, current_round, score, is_completed, attempts, started_at, game_date, rounds
		 FROM sessions WHERE id = $1`, id,
	).Scan(&s.ID, &s.PlayerID, &s.Mode, &s.CurrentRound, &s.Score, &s.IsCompleted, &s.Attempts, &s.StartedAt, &gameDate, &rounds)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.GameDate = gameDate.String
	if rounds != nil {
		if err := json.Unmarshal(rounds, &s.Rounds); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (r *SessionRepo) Update(ctx context.Context, s *domain.Session, answeredRound int) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE sessions SET current_round=$1, score=$2, is_completed=$3, attempts=$4
		 WHERE id=$5 AND current_round=$6 AND is_completed=false`,
		s.CurrentRound, s.Score, s.IsCompleted, s.Attempts, s.ID, answeredRound)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrSessionCompleted
	}
	return nil
}

func (r *SessionRepo) DeleteStale(ctx context.Context, mode domain.GameMode, maxAge time.Duration) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE mode=$1 AND is_completed=false AND started_at < $2`,
		mode, time.Now().Add(-maxAge))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteActiveByPlayer removes the player's incomplete sessions of a
// mode. Called on Free Play replay so the new variant replaces the old
// one immediately instead of waiting for the stale-session GC.
func (r *SessionRepo) DeleteActiveByPlayer(ctx context.Context, playerID string, mode domain.GameMode) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE player_id=$1 AND mode=$2 AND is_completed=false`,
		playerID, mode)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *SessionRepo) TodaySession(ctx context.Context, playerID, date string) (*domain.Session, error) {
	s := &domain.Session{}
	var gd sql.NullString
	var rounds []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT id, player_id, mode, current_round, score, is_completed, attempts, started_at, game_date, rounds
		 FROM sessions WHERE player_id=$1 AND game_date=$2 ORDER BY started_at DESC LIMIT 1`,
		playerID, date,
	).Scan(&s.ID, &s.PlayerID, &s.Mode, &s.CurrentRound, &s.Score, &s.IsCompleted, &s.Attempts, &s.StartedAt, &gd, &rounds)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.GameDate = gd.String
	if rounds != nil {
		if err := json.Unmarshal(rounds, &s.Rounds); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (r *SessionRepo) UpsertStatus(ctx context.Context, playerID, gameDate string, completed bool) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO player_daily_status (player_id, game_date, is_completed)
		 VALUES ($1,$2,$3)
		 ON CONFLICT (game_date, player_id) DO UPDATE SET is_completed = EXCLUDED.is_completed`,
		playerID, gameDate, completed)
	return err
}

func (r *SessionRepo) StatusCompleted(ctx context.Context, playerID, gameDate string) (bool, error) {
	var completed bool
	err := r.db.QueryRowContext(ctx,
		`SELECT is_completed FROM player_daily_status WHERE player_id=$1 AND game_date=$2`,
		playerID, gameDate).Scan(&completed)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return completed, err
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
