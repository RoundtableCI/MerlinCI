package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/RoundtableCI/MerlinCI/internal/artifacts"
	"github.com/RoundtableCI/MerlinCI/internal/budget"
	"github.com/RoundtableCI/MerlinCI/internal/config"
	"github.com/RoundtableCI/MerlinCI/internal/runtime"
	"github.com/RoundtableCI/MerlinCI/pkg/provider"
	"github.com/RoundtableCI/MerlinCI/pkg/tool"
)

// These tests need a real PostgreSQL. Set MERLIN_TEST_DATABASE_URL to run
// them, for example against deploy/docker-compose.yml:
//
//	MERLIN_TEST_DATABASE_URL=postgres://merlin:merlin@localhost:5432/merlin?sslmode=disable
//
// Each test gets a database of its own, dropped when the test ends.
func open(t *testing.T) *Store {
	t.Helper()
	admin := os.Getenv("MERLIN_TEST_DATABASE_URL")
	if admin == "" {
		t.Skip("MERLIN_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	name := fmt.Sprintf("merlin_test_%d", time.Now().UnixNano())
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	cfg := conn.Config().Copy()
	cfg.Database = name
	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", cfg.User, cfg.Password, cfg.Host, cfg.Port, name)

	s, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.Close()
		_, _ = conn.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
		_ = conn.Close(ctx)
	})
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}

var ctx = context.Background()

// compact removes the whitespace PostgreSQL adds when it stores JSON.
func compact(t *testing.T, raw []byte) string {
	t.Helper()
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestMigrateIsRepeatable(t *testing.T) {
	s := open(t)
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	var n int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&n); err != nil || n != 4 {
		t.Fatalf("schema_migrations has %d rows (%v), want 4", n, err)
	}
}

func TestCheckpoints(t *testing.T) {
	c := open(t).Checkpoints()
	if _, found, err := c.Load(ctx, "t1"); err != nil || found {
		t.Fatalf("Load of an unknown task = found %v, %v", found, err)
	}
	state := runtime.State{
		TaskID: "t1", Role: "researcher", Status: runtime.Running, Step: 1, Spent: 126000,
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "go"},
			{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "call_1_0", Name: "search", Arguments: []byte(`{"query":"x"}`)}}},
		},
		PendingCall:  &provider.ToolCall{ID: "call_1_0", Name: "search", Arguments: []byte(`{"query":"x"}`)},
		ConfigDigest: "sha256:abc",
	}
	if err := c.Save(ctx, state); err != nil {
		t.Fatal(err)
	}
	state.Step, state.Status = 2, runtime.AwaitingApproval
	if err := c.Save(ctx, state); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	got, found, err := c.Load(ctx, "t1")
	if err != nil || !found {
		t.Fatalf("Load = found %v, %v", found, err)
	}
	if got.Step != 2 || got.Status != runtime.AwaitingApproval || got.Spent != 126000 || len(got.Messages) != 2 ||
		got.Messages[1].ToolCalls[0].Name != "search" || compact(t, got.PendingCall.Arguments) != `{"query":"x"}` {
		t.Errorf("state did not survive the round trip: %+v", got)
	}
}

func TestEffects(t *testing.T) {
	e := open(t).Effects()
	if got, err := e.Get(ctx, "t1/call_1_0"); err != nil || got != nil {
		t.Fatalf("Get of an unknown call = %+v, %v", got, err)
	}
	if err := e.Finish(ctx, "t1/call_1_0", "x", nil); err == nil {
		t.Error("Finish succeeded for a call that was never begun")
	}

	rec := runtime.Effect{Key: "t1/call_1_0", TaskID: "t1", Tool: "publish", Action: "publish.post"}
	if err := e.Begin(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if err := e.Begin(ctx, rec); err != nil {
		t.Fatalf("second Begin: %v", err)
	}
	got, err := e.Get(ctx, rec.Key)
	if err != nil || got == nil || got.Done || got.Action != "publish.post" {
		t.Fatalf("after Begin: %+v, %v; want a started record", got, err)
	}

	at := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	if err := e.Finish(ctx, rec.Key, "published", []tool.Source{{URL: "https://example.com", RetrievedAt: at}}); err != nil {
		t.Fatal(err)
	}
	// Begin after Finish must not reopen the call.
	if err := e.Begin(ctx, rec); err != nil {
		t.Fatal(err)
	}
	got, err = e.Get(ctx, rec.Key)
	if err != nil || !got.Done || got.Result != "published" || len(got.Sources) != 1 || !got.Sources[0].RetrievedAt.Equal(at) {
		t.Fatalf("after Finish: %+v, %v", got, err)
	}
}

func TestArtifacts(t *testing.T) {
	a := open(t).Artifacts()
	rec := artifacts.Artifact{
		ID: "art_t1", Kind: "market-opportunity", TaskID: "t1", Role: "researcher", Title: "An opportunity",
		Sources:   []artifacts.Source{{URL: "https://example.com", RetrievedAt: "2026-11-13T09:00:00Z"}},
		Cost:      artifacts.Cost{Currency: "INR", Amount: "0.252000"},
		CreatedAt: "2026-11-13T09:02:00Z",
	}
	if err := a.Insert(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if err := a.Insert(ctx, rec); err == nil {
		t.Error("the same artifact was stored twice")
	}
	got, found, err := a.Get(ctx, "art_t1")
	if err != nil || !found || got.Title != rec.Title || got.Cost != rec.Cost || got.Sources[0] != rec.Sources[0] {
		t.Fatalf("Get = %+v, %v, %v", got, found, err)
	}
	if _, found, _ := a.Get(ctx, "art_missing"); found {
		t.Error("found an artifact that was never stored")
	}
}

func deckBudget() config.BudgetSpec {
	return config.BudgetSpec{
		Currency: "INR",
		Limits: []config.Limit{
			{Category: budget.Inference, Period: "day", Amount: 15000},
			{Category: budget.Inference, Period: "month", Amount: 300000},
			{Category: budget.Research, Period: "month", Amount: 50000},
		},
		AlertThresholds: []float64{0.7, 0.9},
	}
}

const rupee = budget.Micros(1_000_000)

func TestLedgerLimitsAndAlerts(t *testing.T) {
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	var alerts []budget.Alert
	l := open(t).Ledger(deckBudget(), func() time.Time { return now }, func(a budget.Alert) { alerts = append(alerts, a) })

	r, err := l.Reserve(ctx, budget.Inference, 100*rupee)
	if err != nil {
		t.Fatal(err)
	}
	var limit *budget.LimitError
	if _, err := l.Reserve(ctx, budget.Inference, 51*rupee); !errors.As(err, &limit) || limit.Period != "day" {
		t.Fatalf("err = %v, want a daily limit error while 100 is held", err)
	}
	if err := l.Settle(ctx, r, 40*rupee); err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(ctx, r, 40*rupee); err != nil {
		t.Fatalf("second Settle: %v", err)
	}
	if spent, _ := l.Spent(ctx, budget.Inference, "day"); spent != 40*rupee {
		t.Fatalf("spent = %s, want 40 (settling twice must charge once)", spent)
	}

	r2, _ := l.Reserve(ctx, budget.Inference, 110*rupee)
	if err := l.Release(ctx, r2); err != nil {
		t.Fatal(err)
	}
	r3, err := l.Reserve(ctx, budget.Inference, 110*rupee)
	if err != nil {
		t.Fatalf("limit still held after release: %v", err)
	}
	if err := l.Settle(ctx, r3, 70*rupee); err != nil { // 110 of 150: past 70%
		t.Fatal(err)
	}
	if len(alerts) != 1 || alerts[0].Threshold != 0.7 || alerts[0].Period != "day" {
		t.Fatalf("alerts = %+v, want one daily alert at 70%%", alerts)
	}

	if _, err := l.Reserve(ctx, "ads", rupee); !errors.Is(err, budget.ErrNoBudget) {
		t.Fatalf("err = %v, want ErrNoBudget", err)
	}

	// A new day frees the daily limit; the month keeps counting.
	now = now.AddDate(0, 0, 1)
	if _, err := l.Reserve(ctx, budget.Inference, 150*rupee); err != nil {
		t.Fatalf("a new day did not reset the daily limit: %v", err)
	}
}

// Many workers racing for the last of a budget must not overspend it.
func TestLedgerDoesNotOverspendUnderConcurrency(t *testing.T) {
	s := open(t)
	const workers = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	granted := 0
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// A ledger per worker, as separate processes would have.
			l := s.Ledger(deckBudget(), nil, nil)
			r, err := l.Reserve(ctx, budget.Inference, 20*rupee)
			if err != nil {
				var limit *budget.LimitError
				if !errors.As(err, &limit) {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			_ = l.Settle(ctx, r, 20*rupee)
			mu.Lock()
			granted++
			mu.Unlock()
		}()
	}
	wg.Wait()
	// 150 per day at 20 a call: exactly 7 calls fit.
	if granted != 7 {
		t.Errorf("%d reservations of 20 were granted against a limit of 150, want 7", granted)
	}
	spent, _ := s.Ledger(deckBudget(), nil, nil).Spent(ctx, budget.Inference, "day")
	if spent != 140*rupee {
		t.Errorf("spent = %s, want 140", spent)
	}
}

func TestOpenReportsABadAddress(t *testing.T) {
	_, err := Open(ctx, "postgres://merlin:wrong@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	if err == nil || !strings.Contains(err.Error(), "connect to PostgreSQL") {
		t.Fatalf("err = %v", err)
	}
}

// Cancelling is what stops a run: once a checkpoint says cancelled, a worker
// still running the task cannot replace it, whether or not the task had
// written a checkpoint before.
func TestCancelledCheckpointCannotBeReplaced(t *testing.T) {
	c := open(t).Checkpoints()
	for _, id := range []string{"started", "never-started"} {
		if id == "started" {
			if err := c.Save(ctx, runtime.State{TaskID: id, Role: "r", Status: runtime.Running, Step: 1}); err != nil {
				t.Fatal(err)
			}
		}
		cancelled := runtime.State{TaskID: id, Role: "r", Status: runtime.Cancelled, Reason: "cancelled by a person"}
		if err := c.Save(ctx, cancelled); err != nil {
			t.Fatalf("%s: cancel: %v", id, err)
		}
		for _, status := range []runtime.Status{runtime.Running, runtime.AwaitingApproval, runtime.Succeeded, runtime.Failed} {
			err := c.Save(ctx, runtime.State{TaskID: id, Role: "r", Status: status, Step: 9})
			if !errors.Is(err, runtime.ErrCancelled) {
				t.Errorf("%s: saving %s over a cancelled run: err = %v, want ErrCancelled", id, status, err)
			}
		}
		got, _, _ := c.Load(ctx, id)
		if got.Status != runtime.Cancelled || got.Step == 9 {
			t.Errorf("%s: state after refused saves = %+v", id, got)
		}
		if err := c.Save(ctx, cancelled); err != nil {
			t.Errorf("%s: cancelling twice: %v", id, err)
		}
	}
}

// The event record and the spend record cannot be rewritten, by anyone.
func TestHistoryIsAppendOnly(t *testing.T) {
	s := open(t)
	if err := s.Events().Record(ctx, runtime.Event{TaskID: "t1", Role: "r", Type: "decision", Data: map[string]any{"approve": false}}); err != nil {
		t.Fatal(err)
	}
	l := s.Ledger(deckBudget(), nil, nil)
	r, _ := l.Reserve(ctx, budget.Inference, rupee)
	if err := l.Settle(ctx, r, rupee); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`UPDATE events SET data = '{"approve": true}'`,
		"DELETE FROM events",
		"TRUNCATE events",
		"UPDATE budget_entries SET amount_micros = 0",
		"DELETE FROM budget_entries",
		"TRUNCATE budget_entries",
	} {
		if _, err := s.pool.Exec(ctx, stmt); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("%q: err = %v, want it refused as append-only", stmt, err)
		}
	}
	events, _ := s.Events().List(ctx, "t1")
	if len(events) != 1 || events[0].Data["approve"] != false {
		t.Errorf("events after the attempts = %+v", events)
	}
	if spent, _ := l.Spent(ctx, budget.Inference, "day"); spent != rupee {
		t.Errorf("spent after the attempts = %s", spent)
	}
}

// A call reserved just before midnight and still in flight counts against
// the new day, so the new day cannot be over-committed while it runs.
func TestHoldAcrossMidnightStillCounts(t *testing.T) {
	now := time.Date(2026, 11, 13, 23, 59, 30, 0, time.UTC)
	l := open(t).Ledger(deckBudget(), func() time.Time { return now }, nil)
	if _, err := l.Reserve(ctx, budget.Inference, 100*rupee); err != nil {
		t.Fatal(err)
	}
	now = time.Date(2026, 11, 14, 0, 0, 30, 0, time.UTC) // a minute later, the next day
	var limit *budget.LimitError
	if _, err := l.Reserve(ctx, budget.Inference, 100*rupee); !errors.As(err, &limit) {
		t.Fatalf("err = %v; want the hold from a minute ago to count against the new day", err)
	}
	if _, err := l.Reserve(ctx, budget.Inference, 50*rupee); err != nil {
		t.Fatalf("the remaining 50 was refused: %v", err)
	}
	// A hold abandoned long ago, by a crash, does not block later days.
	now = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	if _, err := l.Reserve(ctx, budget.Inference, 150*rupee); err != nil {
		t.Fatalf("holds from two days ago still block today: %v", err)
	}
}
