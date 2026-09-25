package cron

import (
	"testing"
	"time"
)

func TestJakartaMidnight(t *testing.T) {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Skipf("tz database unavailable: %v", err)
	}
	// 2026-09-25 17:00 UTC == 2026-09-26 00:00 WIB — at Jakarta midnight.
	now := time.Date(2026, 9, 25, 17, 0, 0, 0, time.UTC)
	got := timeUntilNextMidnightAt(now, jakarta)
	if got != 24*time.Hour {
		t.Fatalf("at Jakarta midnight: got %v, want 24h", got)
	}
	// One hour before Jakarta midnight (16:00 UTC) should wait 1h.
	now = time.Date(2026, 9, 25, 16, 0, 0, 0, time.UTC)
	got = timeUntilNextMidnightAt(now, jakarta)
	if got != time.Hour {
		t.Fatalf("1h before Jakarta midnight: got %v, want 1h", got)
	}
}
