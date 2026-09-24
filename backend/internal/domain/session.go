package domain

import "time"

type GameMode string

const (
	ModeDaily    GameMode = "daily"
	ModeFreePlay GameMode = "freeplay"
)

// Session tracks one play-through. The PK ID is the identifier used in
// API routes (e.g. /api/freeplay/{id}/round/{n}).
type Session struct {
	ID           string    `json:"id"`
	PlayerID     string    `json:"player_id"`
	Mode         GameMode  `json:"mode"`
	CurrentRound int       `json:"current_round"`
	Score        int       `json:"score"`
	IsCompleted  bool      `json:"is_completed"`
	Attempts     int       `json:"attempts"` // attempts spent on the current round
	StartedAt    time.Time `json:"started_at"`
	GameDate     string    `json:"game_date,omitempty"` // daily only, YYYY-MM-DD
}

// Completed reports whether all rounds have been answered. A daily
// session locks (and becomes unresumable) only once this is reached.
func (s *Session) Completed() bool {
	return s.CurrentRound > RoundsPerGame
}
