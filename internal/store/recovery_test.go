package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/RoundtableCI/MerlinCI/internal/artifacts"
	"github.com/RoundtableCI/MerlinCI/internal/blob"
	"github.com/RoundtableCI/MerlinCI/internal/budget"
	"github.com/RoundtableCI/MerlinCI/internal/config"
	"github.com/RoundtableCI/MerlinCI/internal/eval"
	"github.com/RoundtableCI/MerlinCI/internal/gateway"
	"github.com/RoundtableCI/MerlinCI/internal/policy"
	"github.com/RoundtableCI/MerlinCI/internal/runtime"
	"github.com/RoundtableCI/MerlinCI/internal/tools"
	"github.com/RoundtableCI/MerlinCI/pkg/evaluator"
	"github.com/RoundtableCI/MerlinCI/pkg/provider"
	"github.com/RoundtableCI/MerlinCI/pkg/tool"
)

// world is everything outside the process: the model provider and the
// service a tool talks to. It survives a restart; the process does not.
type world struct {
	script        []any // provider.Message or error
	inference     int
	searches      int
	publishes     int
	diePublishing bool
}

func (w *world) Name() string { return "fake" }

func (w *world) Complete(context.Context, provider.Request) (provider.Response, error) {
	if w.inference >= len(w.script) {
		return provider.Response{}, errors.New("script exhausted")
	}
	next := w.script[w.inference]
	w.inference++
	if err, ok := next.(error); ok {
		return provider.Response{}, err
	}
	return provider.Response{Message: next.(provider.Message), Usage: provider.Usage{InputTokens: 1000, OutputTokens: 200}}, nil
}

type worldTool struct {
	w            *world
	name, action string
}

func (t worldTool) Spec() tool.Spec {
	return tool.Spec{Name: t.name, Action: t.action, InputSchema: json.RawMessage(`{"type":"object"}`)}
}

func (t worldTool) Call(context.Context, json.RawMessage) (tool.Result, error) {
	if t.name == "publish" {
		t.w.publishes++
		if t.w.diePublishing {
			t.w.diePublishing = false
			panic("process killed mid-publish")
		}
		return tool.Result{Content: "published"}, nil
	}
	t.w.searches++
	return tool.Result{
		Content: "a thread",
		Sources: []tool.Source{{URL: "https://example.com/thread", RetrievedAt: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)}},
	}, nil
}

func bundle() *config.Bundle {
	return &config.Bundle{
		Digest: "sha256:" + strings.Repeat("0", 64),
		Profile: &config.Profile{Spec: config.ProfileSpec{
			Approval: config.Approval{Status: "approved", ApprovedBy: "a person", ApprovedOn: "2026-11-13"},
			Company:  config.Company{Name: "Example Co"},
		}},
		Policy: &config.Policy{Spec: config.PolicySpec{Default: "deny", Rules: []config.PolicyRule{
			{Action: "research.*", Tier: config.TierAutomatic},
			{Action: "publish.*", Tier: config.TierExplicit, Reason: "publishing"},
		}}},
		Budget: &config.Budget{Spec: config.BudgetSpec{
			Currency:      "INR",
			Limits:        []config.Limit{{Category: "inference", Period: "day", Amount: 15000}},
			ExchangeRates: map[string]config.Rate{"USD": 900000},
		}},
		Roles: map[string]config.Role{"researcher": {Spec: config.RoleSpec{
			Enabled:     true,
			Tools:       []string{"search", "publish"},
			Permissions: []string{"research.search", "publish.post"},
			Output:      config.RoleOutput{ArtifactKind: "market-opportunity"},
			Evaluation:  config.Evaluation{Evaluators: []string{"evidence"}},
			Limits:      config.RoleLimits{MaxSteps: 6, MaxCostPerTask: 2500},
		}}},
	}
}

// boot starts a fresh process: a new runtime with nothing in memory, wired
// to the same database and the same outside world.
func boot(t *testing.T, s *Store, w *world) *runtime.Runtime {
	t.Helper()
	cfg := bundle()
	registry := tools.NewRegistry()
	for _, tl := range []tool.Tool{worldTool{w, "search", "research.search"}, worldTool{w, "publish", "publish.post"}} {
		if err := registry.Register(tl); err != nil {
			t.Fatal(err)
		}
	}
	rate, err := gateway.RateFor(cfg.Budget.Spec, "USD")
	if err != nil {
		t.Fatal(err)
	}
	gw, err := gateway.New(gateway.Config{
		Provider: w,
		Models:   []gateway.Model{{ID: "test-model", InputPerMTok: 1_000_000, OutputPerMTok: 2_000_000}},
		Currency: "USD",
		Rate:     rate,
		Ledger:   s.Ledger(cfg.Budget.Spec, nil, nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &runtime.Runtime{
		Config:      cfg,
		Gateway:     gw,
		Tools:       registry,
		Policy:      policy.New(cfg.Policy.Spec, nil),
		Evaluators:  map[string]evaluator.Evaluator{"evidence": eval.Evidence{}},
		Artifacts:   &artifacts.Store{Blobs: blob.NewMemory(), Index: s.Artifacts()},
		Checkpoints: s.Checkpoints(),
		Effects:     s.Effects(),
	}
}

func toolCall(name string) provider.Message {
	return provider.Message{ToolCalls: []provider.ToolCall{{Name: name, Arguments: json.RawMessage(`{}`)}}}
}

var researchTask = runtime.Task{ID: "t1", Role: "researcher", Input: map[string]any{"topic": "model comparison"}}

// Phase 0, on the real database: an agent takes a task, uses a tool, is
// interrupted, and a new process finishes the job from the last checkpoint
// without repeating the tool call. The artifact and the spend are in
// PostgreSQL at the end.
func TestRunSurvivesARestart(t *testing.T) {
	s := open(t)
	w := &world{script: []any{toolCall("search"), fmt.Errorf("%w: connection refused", provider.ErrNotBilled), provider.Message{Content: "# An opportunity\nx"}}}

	if _, err := boot(t, s, w).Run(ctx, researchTask); err == nil {
		t.Fatal("the first process finished although its connection was reset")
	}

	state, err := boot(t, s, w).Run(ctx, researchTask)
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != runtime.Succeeded {
		t.Fatalf("status = %s (%s)", state.Status, state.Reason)
	}
	if w.searches != 1 {
		t.Errorf("search ran %d times across the restart, want 1", w.searches)
	}
	rec, found, err := s.Artifacts().Get(ctx, state.ArtifactID)
	if err != nil || !found || len(rec.Sources) != 1 || rec.Cost.Amount != "0.252000" {
		t.Fatalf("artifact in PostgreSQL = %+v, found %v, %v", rec, found, err)
	}
	spent, err := s.Ledger(bundle().Budget.Spec, nil, nil).Spent(ctx, budget.Inference, "day")
	if err != nil || spent != 252000 {
		t.Errorf("spend in PostgreSQL = %d, %v; want 252000 (the failed call is not charged)", spent, err)
	}

	// A third process is asked to run the finished task: nothing happens.
	again, err := boot(t, s, w).Run(ctx, researchTask)
	if err != nil || again.ArtifactID != state.ArtifactID || w.inference != 3 || w.searches != 1 {
		t.Errorf("a finished task ran again: %v, %d inference calls, %d searches", err, w.inference, w.searches)
	}
}

// Publishing actions are never repeated on recovery (deck slide 10), on the
// real database: the process is killed mid-publish, and the next process
// waits for a person instead of publishing a second time.
func TestPublishIsNotRepeatedAfterARestart(t *testing.T) {
	s := open(t)
	w := &world{
		script:        []any{toolCall("publish"), toolCall("search"), provider.Message{Content: "# Published\nx"}},
		diePublishing: true,
	}

	first := boot(t, s, w)
	state, err := first.Run(ctx, researchTask)
	if err != nil || state.Status != runtime.AwaitingApproval {
		t.Fatalf("state = %s, %v; want the publish to wait for approval", state.Status, err)
	}
	func() {
		defer func() { _ = recover() }()
		_, _ = first.Approve(ctx, "t1")
	}()
	if w.publishes != 1 {
		t.Fatalf("publish was attempted %d times, want 1", w.publishes)
	}

	second := boot(t, s, w)
	state, err = second.Run(ctx, researchTask)
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != runtime.AwaitingApproval || !strings.Contains(state.Reason, "interrupted and may already have happened") {
		t.Fatalf("after restart: %s (%s); want the run to wait for a person", state.Status, state.Reason)
	}
	if w.publishes != 1 {
		t.Fatalf("publish was attempted %d times after the restart, want still 1", w.publishes)
	}

	state, err = second.Approve(ctx, "t1")
	if err != nil || state.Status != runtime.Succeeded || w.publishes != 2 {
		t.Fatalf("after a person approved again: %s, %v, %d publish attempts; want succeeded and 2", state.Status, err, w.publishes)
	}
}
