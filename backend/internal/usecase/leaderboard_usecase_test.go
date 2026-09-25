package usecase

import (
	"context"
	"testing"
	"time"

	"movie-trivia/internal/domain"
)

// Regression for the step-6 leaderboard flow: submit requires a
// completed daily run, validates the name, and is one-shot per player
// per day (the UNIQUE constraint's application-level guard).
func TestLeaderboardSubmitFlow(t *testing.T) {
	repo := newFakeSessionRepo()
	entries := newFakeLeaderboardRepo()
	u := NewLeaderboardUsecase(entries, repo, time.UTC)

	// Not completed yet → forbidden.
	if err := u.Submit(context.Background(), "player-1", "Alice"); err != domain.ErrNotSubmitted {
		t.Fatalf("before completion: want ErrNotSubmitted, got %v", err)
	}

	// Mark today's run completed for the player.
	gameDate := time.Now().UTC().Format("2006-01-02")
	if err := repo.UpsertStatus(context.Background(), "player-1", gameDate, true); err != nil {
		t.Fatal(err)
	}

	if err := u.Submit(context.Background(), "player-1", "  Alice  "); err != nil {
		t.Fatalf("valid submit: %v", err)
	}
	if err := u.Submit(context.Background(), "player-1", "Alice"); err != domain.ErrDuplicateSubmit {
		t.Fatalf("second submit: want ErrDuplicateSubmit, got %v", err)
	}
	if err := u.Submit(context.Background(), "player-2", "This name is way too long!!"); err != domain.ErrInvalidName {
		t.Fatalf("long name: want ErrInvalidName, got %v", err)
	}

	top, err := u.Top10(context.Background())
	if err != nil || len(top) != 1 || top[0].Name != "Alice" {
		t.Fatalf("top10: %+v, %v", top, err)
	}
	if top[0].Score != 42 {
		t.Fatalf("entry must carry the session's final score, got %d", top[0].Score)
	}
}
