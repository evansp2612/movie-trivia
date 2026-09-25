package cron

import (
	"testing"
	"time"
)

func TestTimeUntilNextMidnight(t *testing.T) {
	loc := time.FixedZone("test", 0) // UTC-equivalent for determinism

	cases := []struct {
		name string
		now  string // RFC3339 in loc
		want string // duration until next midnight
	}{
		{"midday", "2026-09-25T12:00:00Z", "12h0m0s"},
		{"just after midnight", "2026-09-25T00:00:01Z", "23h59m59s"},
		{"just before midnight", "2026-09-25T23:59:59Z", "1s"},
		{"exactly midnight", "2026-09-25T00:00:00Z", "24h0m0s"},
		{"month rollover", "2026-09-30T18:30:00Z", "5h30m0s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(
				mustTime(t, tc.now).Year(), mustTime(t, tc.now).Month(), mustTime(t, tc.now).Day(),
				mustTime(t, tc.now).Hour(), mustTime(t, tc.now).Minute(), mustTime(t, tc.now).Second(), 0, loc)
			// Fake the clock by computing against a fixed "now".
			got := timeUntilNextMidnightAt(now, loc)
			want := mustDuration(t, tc.want)
			if got != want {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func mustDuration(t *testing.T, s string) time.Duration {
	t.Helper()
	d, err := time.ParseDuration(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
