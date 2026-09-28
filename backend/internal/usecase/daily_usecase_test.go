package usecase

import (
	"context"
	"testing"
	"time"

	"movie-trivia/internal/domain"
)

// The end screen's own-entry highlight is anchored to the backend's
// my_entry (name + score) rather than a locally remembered name, so a
// rival with the same name (but a different score) can't steal or
// duplicate the highlight.
func TestResultIncludesMyEntry(t *testing.T) {
	repo := newFakeSessionRepo()
	entries := newFakeLeaderboardRepo()
	today := time.Now().UTC().Format("2006-01-02")

	// playerA completed today's run (fake serves score 42) and submitted;
	// a rival shares the name but sits on the board with another score.
	if err := repo.UpsertStatus(context.Background(), playerA, today, true); err != nil {
		t.Fatal(err)
	}
	entries.submitted = []domain.LeaderboardEntry{
		{GameDate: today, PlayerID: "rival-id", Name: "Tester", Score: 10},
		{GameDate: today, PlayerID: playerA, Name: "Tester", Score: 42},
	}

	u := NewDailyUsecase(repo, nil, entries, nil, time.UTC)
	res, err := u.Result(context.Background(), playerA)
	if err != nil {
		t.Fatalf("result: %v", err)
	}
	m := res.(map[string]any)
	if m["score"] != 42 || m["game_date"] != today {
		t.Fatalf("score/game_date: %+v", m)
	}
	if m["submitted"] != true {
		t.Fatalf("submitted must be true, got %+v", m)
	}
	me, ok := m["my_entry"].(map[string]any)
	if !ok || me["name"] != "Tester" || me["score"] != 42 || me["player_id"] != playerA {
		t.Fatalf("my_entry must anchor to the player's own submission, got %+v", m["my_entry"])
	}
	if board, ok := m["leaderboard"].([]domain.LeaderboardEntry); !ok || len(board) != 2 {
		t.Fatalf("leaderboard must carry the day's board, got %+v", m["leaderboard"])
	}

	// Completed but not submitted: submitted=false, no my_entry.
	if err := repo.UpsertStatus(context.Background(), playerB, today, true); err != nil {
		t.Fatal(err)
	}
	res, err = u.Result(context.Background(), playerB)
	if err != nil {
		t.Fatalf("result (unsubmitted): %v", err)
	}
	m = res.(map[string]any)
	if m["submitted"] != false {
		t.Fatalf("unsubmitted player: submitted must be false, got %+v", m)
	}
	if _, has := m["my_entry"]; has {
		t.Fatalf("unsubmitted player must have no my_entry, got %+v", m)
	}

	// No session at all → not found.
	if _, err := u.Result(context.Background(), "nobody"); err != domain.ErrNotFound {
		t.Fatalf("no session: want ErrNotFound, got %v", err)
	}
}
