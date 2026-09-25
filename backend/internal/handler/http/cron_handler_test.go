package httphandler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

type fakeForceRunner struct{ calls int }

func (f *fakeForceRunner) ForceRun(context.Context) (any, error) {
	f.calls++
	return map[string]any{"pool_count": 99, "game_status": "generated", "gc_removed": 2}, nil
}

func TestCronDailyAuth(t *testing.T) {
	handler := NewCronHandler(&fakeForceRunner{}, "my-secret", "admin-pass")

	// No credentials → 401.
	r := httptest.NewRequest("GET", "/api/cron/daily", nil)
	w := httptest.NewRecorder()
	handler.Daily(w, r)
	if w.Code != 401 {
		t.Fatalf("no credentials: got %d, want 401", w.Code)
	}

	// Wrong bearer → 401.
	r = httptest.NewRequest("GET", "/api/cron/daily", nil)
	r.Header.Set("Authorization", "Bearer wrong")
	w = httptest.NewRecorder()
	handler.Daily(w, r)
	if w.Code != 401 {
		t.Fatalf("wrong bearer: got %d, want 401", w.Code)
	}

	// Correct bearer (Vercel Cron style) → 200 + summary.
	r = httptest.NewRequest("GET", "/api/cron/daily", nil)
	r.Header.Set("Authorization", "Bearer my-secret")
	w = httptest.NewRecorder()
	handler.Daily(w, r)
	if w.Code != 200 {
		t.Fatalf("valid bearer: got %d, want 200", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["pool_count"].(float64) != 99 || body["game_status"] != "generated" || body["gc_removed"].(float64) != 2 {
		t.Fatalf("summary mismatch: %v", body)
	}

	// Admin password header (cron-job.org style) → 200.
	r = httptest.NewRequest("GET", "/api/cron/daily", nil)
	r.Header.Set("X-Admin-Password", "admin-pass")
	w = httptest.NewRecorder()
	handler.Daily(w, r)
	if w.Code != 200 {
		t.Fatalf("admin header: got %d, want 200", w.Code)
	}
}
