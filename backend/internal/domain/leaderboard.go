package domain

import "time"

type LeaderboardEntry struct {
	GameDate    string    `json:"game_date"`
	PlayerID    string    `json:"player_id"`
	Name        string    `json:"name"`
	Score       int       `json:"score"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// MaxNameLength caps the display name players submit.
const MaxNameLength = 20
