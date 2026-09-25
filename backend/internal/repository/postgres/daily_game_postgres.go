package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"movie-trivia/internal/domain"
)

type DailyGameRepo struct{ db *sql.DB }

func NewDailyGameRepo(db *sql.DB) *DailyGameRepo { return &DailyGameRepo{db: db} }

// Upsert stores today's fixed 10-round set. UNIQUE(game_date) plus
// ON CONFLICT DO NOTHING prevents concurrent cron generation from
// producing divergent sets.
func (r *DailyGameRepo) Upsert(ctx context.Context, gameDate string, rounds []domain.Round) error {
	b, err := json.Marshal(rounds)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO daily_games (game_date, rounds) VALUES ($1,$2)
		 ON CONFLICT (game_date) DO NOTHING`, gameDate, b)
	return err
}

func (r *DailyGameRepo) Get(ctx context.Context, gameDate string) ([]domain.Round, error) {
	var b []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT rounds FROM daily_games WHERE game_date=$1`, gameDate).Scan(&b)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var rounds []domain.Round
	return rounds, json.Unmarshal(b, &rounds)
}

// Has reports whether a round set exists for the date — used by the
// daily chain's self-healing re-trigger to detect a midnight run that
// died before generating the game.
func (r *DailyGameRepo) Has(ctx context.Context, gameDate string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM daily_games WHERE game_date=$1)`, gameDate).Scan(&exists)
	return exists, err
}
