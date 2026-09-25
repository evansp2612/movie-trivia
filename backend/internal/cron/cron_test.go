package cron

import (
	"context"
	"errors"
	"testing"
	"time"

	"movie-trivia/internal/domain"
)

// ---- fakes ----

type fakeMarker struct {
	claimed map[string]bool
	failErr error
}

func newFakeMarker() *fakeMarker { return &fakeMarker{claimed: map[string]bool{}} }

func (f *fakeMarker) ClaimDailyRun(_ context.Context, date string) (bool, error) {
	if f.failErr != nil {
		return false, f.failErr
	}
	if f.claimed[date] {
		return false, nil
	}
	f.claimed[date] = true
	return true, nil
}

type fakeGCRepo struct{ sweeps int }

func (f *fakeGCRepo) DeleteStale(context.Context, domain.GameMode, time.Duration) (int64, error) {
	f.sweeps++
	return 3, nil
}

type fakePool struct {
	refreshed int
	failErr   error
}

func (f *fakePool) Refresh(context.Context) ([]domain.Movie, error) {
	f.refreshed++
	if f.failErr != nil {
		return nil, f.failErr
	}
	return []domain.Movie{{TMDBID: 1, Title: "A", Year: 2000, IMDBRating: 7}}, nil
}

type fakeBuilder struct{ built int }

func (f *fakeBuilder) BuildRounds(context.Context) ([]domain.Round, error) {
	f.built++
	return []domain.Round{{Index: 1, Type: domain.RoundGuessYear}}, nil
}

type fakeGames struct {
	upserts []string
	has     bool
}

func (f *fakeGames) Upsert(_ context.Context, gameDate string, _ []domain.Round) error {
	f.upserts = append(f.upserts, gameDate)
	f.has = true
	return nil
}

func (f *fakeGames) Has(context.Context, string) (bool, error) { return f.has, nil }

type fakeGenLock struct{ acquired int }

func (f *fakeGenLock) AcquireGenLock(context.Context, time.Duration) (bool, error) {
	f.acquired++
	return true, nil
}
func (f *fakeGenLock) ReleaseGenLock(context.Context) error { return nil }

// ---- tests ----

func TestRunDailyRunsAllStepsInOrder(t *testing.T) {
	pool := &fakePool{}
	builder := &fakeBuilder{}
	games := &fakeGames{}
	gc := &fakeGCRepo{}
	marker := newFakeMarker()
	s := NewScheduler(pool, builder, games, &fakeGenLock{}, gc, marker, time.UTC)

	if _, err := s.RunDaily(context.Background()); err != nil {
		t.Fatal(err)
	}
	if pool.refreshed != 1 {
		t.Fatalf("pool refreshed %d times", pool.refreshed)
	}
	if builder.built != 1 {
		t.Fatalf("rounds built %d times", builder.built)
	}
	if len(games.upserts) != 1 {
		t.Fatalf("game upserted %d times", len(games.upserts))
	}
	if gc.sweeps != 1 {
		t.Fatalf("gc swept %d times", gc.sweeps)
	}
	if _, ok := marker.claimed[time.Now().UTC().Format("2006-01-02")]; ok {
		// RunDaily itself must not claim the marker — that's RunIfNeeded's job.
		t.Fatal("RunDaily should not touch the day marker")
	}
}

func TestRunDailyToleratesFailingPool(t *testing.T) {
	pool := &fakePool{failErr: errors.New("tmdb down")}
	builder := &fakeBuilder{}
	games := &fakeGames{}
	gc := &fakeGCRepo{}
	s := NewScheduler(pool, builder, games, &fakeGenLock{}, gc, newFakeMarker(), time.UTC)

	if _, err := s.RunDaily(context.Background()); err != nil {
		t.Fatal(err)
	}
	if builder.built != 1 || gc.sweeps != 1 {
		t.Fatalf("later steps must still run: built=%d gc=%d", builder.built, gc.sweeps)
	}
	if len(games.upserts) != 1 {
		t.Fatalf("game must still generate: %v", games.upserts)
	}
}

func TestRunIfNeededClaimsOncePerDate(t *testing.T) {
	marker := newFakeMarker()
	s := NewScheduler(&fakePool{}, &fakeBuilder{}, &fakeGames{}, &fakeGenLock{}, &fakeGCRepo{}, marker, time.UTC)

	s.RunIfNeeded(context.Background())
	s.RunIfNeeded(context.Background()) // same process: sync.Once + marker both say no

	if len(marker.claimed) != 1 {
		t.Fatalf("expected exactly one claimed date, got %v", marker.claimed)
	}
}

func TestSelfHealRerunsWhenGameMissing(t *testing.T) {
	pool := &fakePool{}
	builder := &fakeBuilder{}
	games := &fakeGames{} // has=false: the midnight run died before generating
	gc := &fakeGCRepo{}
	marker := newFakeMarker()
	s := NewScheduler(pool, builder, games, &fakeGenLock{}, gc, marker, time.UTC)

	// Marker already claimed (simulating a failed midnight run)...
	if _, err := marker.ClaimDailyRun(context.Background(), time.Now().UTC().Format("2006-01-02")); err != nil {
		t.Fatal(err)
	}
	// ...then RunIfNeeded must still self-heal by re-running the chain.
	s.RunIfNeeded(context.Background())
	time.Sleep(100 * time.Millisecond) // the re-run is async

	if pool.refreshed < 1 || builder.built < 1 {
		t.Fatalf("self-heal should have re-run the chain: pool=%d built=%d", pool.refreshed, builder.built)
	}
	if len(games.upserts) == 0 {
		t.Fatal("self-heal should have generated the game")
	}
}

func TestSelfHealSkipsWhenGamePresent(t *testing.T) {
	pool := &fakePool{}
	builder := &fakeBuilder{}
	games := &fakeGames{has: true}
	gc := &fakeGCRepo{}
	marker := newFakeMarker()
	s := NewScheduler(pool, builder, games, &fakeGenLock{}, gc, marker, time.UTC)

	if _, err := marker.ClaimDailyRun(context.Background(), time.Now().UTC().Format("2006-01-02")); err != nil {
		t.Fatal(err)
	}
	s.RunIfNeeded(context.Background())
	time.Sleep(100 * time.Millisecond)

	if pool.refreshed != 0 || builder.built != 0 {
		t.Fatalf("game present: chain must not re-run (pool=%d built=%d)", pool.refreshed, builder.built)
	}
}
