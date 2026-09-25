package usecase

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"movie-trivia/internal/domain"
)

// ---- fakes ----

type fakeSessionRepo struct {
	created       []*domain.Session
	deletedActive []domain.GameMode
	byID          map[string]*domain.Session
	completed     map[string]bool // "playerID|gameDate" -> completed
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{byID: map[string]*domain.Session{}, completed: map[string]bool{}}
}

func (f *fakeSessionRepo) Create(_ context.Context, s *domain.Session) error {
	s.ID = "sess-" + string(rune(len(f.created)+'a'))
	cp := *s
	f.created = append(f.created, &cp)
	f.byID[s.ID] = &cp
	return nil
}

func (f *fakeSessionRepo) Get(_ context.Context, id string) (*domain.Session, error) {
	if s, ok := f.byID[id]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeSessionRepo) Update(_ context.Context, s *domain.Session, _ int) error { return nil }
func (f *fakeSessionRepo) DeleteStale(_ context.Context, _ domain.GameMode, _ time.Duration) (int64, error) {
	return 0, nil
}
func (f *fakeSessionRepo) DeleteActiveByPlayer(_ context.Context, _ string, mode domain.GameMode) (int64, error) {
	f.deletedActive = append(f.deletedActive, mode)
	return 1, nil
}
func (f *fakeSessionRepo) TodaySession(_ context.Context, playerID, date string) (*domain.Session, error) {
	if f.completed[playerID+"|"+date] {
		return &domain.Session{PlayerID: playerID, GameDate: date, IsCompleted: true, Score: 42}, nil
	}
	return nil, domain.ErrNotFound
}
func (f *fakeSessionRepo) UpsertStatus(_ context.Context, playerID, gameDate string, completed bool) error {
	f.completed[playerID+"|"+gameDate] = completed
	return nil
}
func (f *fakeSessionRepo) StatusCompleted(_ context.Context, playerID, gameDate string) (bool, error) {
	return f.completed[playerID+"|"+gameDate], nil
}

type fakeRoundBuilder struct {
	GameUsecase
	rounds []domain.Round
	err    error
}

func newFakeRoundBuilder() *fakeRoundBuilder {
	return &fakeRoundBuilder{GameUsecase: GameUsecase{rng: rand.New(rand.NewSource(7))}}
}

func (f *fakeRoundBuilder) BuildRounds(context.Context) ([]domain.Round, error) {
	return f.rounds, f.err
}

func testRounds() []domain.Round {
	rounds := make([]domain.Round, domain.RoundsPerGame)
	for i := range rounds {
		rounds[i] = domain.Round{Index: i + 1, Type: domain.RoundGuessYear, YearMin: 1950, YearMax: 2026}
	}
	return rounds
}

const playerA = "aaaaaaaa-1111-1111-1111-111111111111"
const playerB = "bbbbbbbb-2222-2222-2222-222222222222"

func newTestUsecase(t *testing.T) (*FreeplayUsecase, *fakeSessionRepo) {
	t.Helper()
	repo := newFakeSessionRepo()
	builder := newFakeRoundBuilder()
	builder.rounds = testRounds()
	return NewFreeplayUsecase(repo, builder), repo
}

// ---- tests ----

func TestStartPersistsVariantAndReplacesPrevious(t *testing.T) {
	u, repo := newTestUsecase(t)

	s1, err := u.Start(context.Background(), playerA)
	if err != nil {
		t.Fatal(err)
	}
	if len(s1.Rounds) != domain.RoundsPerGame {
		t.Fatalf("session should carry %d rounds, got %d", domain.RoundsPerGame, len(s1.Rounds))
	}
	if s1.Rounds[0].Index != 1 || s1.Rounds[9].Index != 10 {
		t.Fatal("rounds should be indexed 1..10")
	}

	// Replay deletes the previous active variant before creating the new one.
	s2, err := u.Start(context.Background(), playerA)
	if err != nil {
		t.Fatal(err)
	}
	if s1.ID == s2.ID {
		t.Fatal("replay must create a new session")
	}
	if len(repo.deletedActive) != 2 || repo.deletedActive[1] != domain.ModeFreePlay {
		t.Fatalf("expected a DeleteActiveByPlayer(freeplay) call per Start, got %v", repo.deletedActive)
	}
}

func TestRoundServesStoredVariant(t *testing.T) {
	u, repo := newTestUsecase(t)
	s, _ := u.Start(context.Background(), playerA)

	for _, n := range []int{1, 5, 10} {
		got, err := u.Round(context.Background(), playerA, s.ID, n)
		if err != nil {
			t.Fatalf("round %d: %v", n, err)
		}
		if got.Index != n {
			t.Fatalf("round %d: got index %d", n, got.Index)
		}
	}

	// Out of range.
	if _, err := u.Round(context.Background(), playerA, s.ID, 0); err != domain.ErrNotFound {
		t.Fatalf("round 0: want ErrNotFound, got %v", err)
	}
	if _, err := u.Round(context.Background(), playerA, s.ID, 11); err != domain.ErrNotFound {
		t.Fatalf("round 11: want ErrNotFound, got %v", err)
	}

	// Unknown session.
	if _, err := u.Round(context.Background(), playerA, "nope", 1); err != domain.ErrNotFound {
		t.Fatalf("unknown session: want ErrNotFound, got %v", err)
	}

	_ = repo
}

func TestRoundForeignSessionIsNotFound(t *testing.T) {
	u, _ := newTestUsecase(t)
	s, _ := u.Start(context.Background(), playerA)

	if _, err := u.Round(context.Background(), playerB, s.ID, 1); err != domain.ErrNotFound {
		t.Fatalf("foreign player must get ErrNotFound, got %v", err)
	}
}

func TestRoundCompletedSessionRejected(t *testing.T) {
	u, repo := newTestUsecase(t)
	s, _ := u.Start(context.Background(), playerA)
	repo.byID[s.ID].IsCompleted = true

	if _, err := u.Round(context.Background(), playerA, s.ID, 1); err != domain.ErrSessionCompleted {
		t.Fatalf("completed session: want ErrSessionCompleted, got %v", err)
	}
}
