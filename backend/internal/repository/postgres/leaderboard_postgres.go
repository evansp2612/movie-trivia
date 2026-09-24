package postgres

import (
	"context"
	"database/sql"

	"movie-trivia/internal/domain"
)

type LeaderboardRepo struct{ db *sql.DB }

func NewLeaderboardRepo(db *sql.DB) *LeaderboardRepo { return &LeaderboardRepo{db: db} }

// Submit inserts one entry; the UNIQUE(game_date, player_id) constraint
// enforces a single submission per player per day.
func (r *LeaderboardRepo) Submit(ctx context.Context, e *domain.LeaderboardEntry) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO leaderboard_entries (game_date, player_id, name, score, submitted_at)
		 VALUES ($1,$2,$3,$4,$5)`,
		e.GameDate, e.PlayerID, e.Name, e.Score, e.SubmittedAt)
	return err
}

// Top10 orders by score DESC, submitted_at ASC — ties rank by whoever
// achieved the score first.
func (r *LeaderboardRepo) Top10(ctx context.Context, gameDate string) ([]domain.LeaderboardEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT game_date, player_id, name, score, submitted_at
		 FROM leaderboard_entries WHERE game_date=$1
		 ORDER BY score DESC, submitted_at ASC LIMIT 10`, gameDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.LeaderboardEntry
	for rows.Next() {
		var e domain.LeaderboardEntry
		if err := rows.Scan(&e.GameDate, &e.PlayerID, &e.Name, &e.Score, &e.SubmittedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *LeaderboardRepo) HasSubmitted(ctx context.Context, gameDate, playerID string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM leaderboard_entries WHERE game_date=$1 AND player_id=$2`,
		gameDate, playerID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}
