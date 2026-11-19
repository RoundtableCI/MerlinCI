package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"go.temporal.io/sdk/testsuite"

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

type world struct {
	script    []any // provider.Message or error
	inference int
	searches  int
	publishes int
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
		return tool.Result{Content: "published"}, nil
	}
	t.w.searches++
	return tool.Result{Content: "a thread", Sources: []tool.Source{{URL: "https://example.com/thread", RetrievedAt: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)}}}, nil
}

func newRuntime(t *testing.T, w *world, enabled bool) *runtime.Runtime {
	t.Helper()
	cfg := &config.Bundle{
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
			Enabled:     enabled,
			Tools:       []string{"search", "publish"},
			Permissions: []string{"research.search", "publish.post"},
			Output:      config.RoleOutput{ArtifactKind: "market-opportunity"},
			Evaluation:  config.Evaluation{Evaluators: []string{"evidence"}},
			Limits:      config.RoleLimits{MaxSteps: 8, MaxCostPerTask: 2500},
		}}},
	}
	registry := tools.NewRegistry()
	for _, tl := range []tool.Tool{worldTool{w, "search", "research.search"}, worldTool{w, "publish", "publish.post"}} {
		if err := registry.Register(tl); err != nil {
			t.Fatal(err)
		}
	}
	rate, _ := gateway.RateFor(cfg.Budget.Spec, "USD")
	gw, err := gateway.New(gateway.Config{
		Provider: w,
		Models:   []gateway.Model{{ID: "test-model", InputPerMTok: 1_000_000, OutputPerMTok: 2_000_000}},
		Currency: "USD",
		Rate:     rate,
		Ledger:   budget.NewLedger(cfg.Budget.Spec, nil, nil),
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
		Artifacts:   &artifacts.Store{Blobs: blob.NewMemory(), Index: artifacts.NewMemoryIndex()},
		Checkpoints: runtime.NewMemoryCheckpoints(),
		Effects:     runtime.NewMemoryEffects(),
	}
}

func toolCall(name string) provider.Message {
	return provider.Message{ToolCalls: []provider.ToolCall{{Name: name, Arguments: json.RawMessage(`{}`)}}}
}

func done(text string) provider.Message { return provider.Message{Content: text} }

var task = runtime.Task{ID: "t1", Role: "researcher", Input: map[string]any{"topic": "model comparison"}}

func newEnv(t *testing.T, w *world, enabled bool) *testsuite.TestWorkflowEnvironment {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivity(&Activities{Runtime: newRuntime(t, w, enabled)})
	return env
}

func finished(t *testing.T, env *testsuite.TestWorkflowEnvironment) Result {
	t.Helper()
	if !env.IsWorkflowCompleted() {
		t.Fatal("the workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}
	var res Result
	if err := env.GetWorkflowResult(&res); err != nil {
		t.Fatal(err)
	}
	return res
}

func TestWorkflowRunsATask(t *testing.T) {
	w := &world{script: []any{toolCall("search"), done("# An opportunity\nx")}}
	env := newEnv(t, w, true)
	env.ExecuteWorkflow(Task, task)

	res := finished(t, env)
	if res.Status != runtime.Succeeded || res.ArtifactID != "art_t1" || res.Spent != "0.252000" {
		t.Fatalf("result = %+v", res)
	}
}

// A failed attempt is retried by Temporal, and the retry continues the run
// from its checkpoint rather than starting it again.
func TestWorkflowRetriesAndResumes(t *testing.T) {
	w := &world{script: []any{toolCall("search"), errors.New("connection reset"), done("# An opportunity\nx")}}
	env := newEnv(t, w, true)
	env.ExecuteWorkflow(Task, task)

	res := finished(t, env)
	if res.Status != runtime.Succeeded {
		t.Fatalf("result = %+v", res)
	}
	if w.searches != 1 || w.inference != 3 {
		t.Errorf("%d searches and %d inference calls; want 1 and 3", w.searches, w.inference)
	}
}

func TestWorkflowDoesNotRetryWhatCannotRun(t *testing.T) {
	w := &world{}
	env := newEnv(t, w, false) // the role is disabled
	env.ExecuteWorkflow(Task, task)

	if err := env.GetWorkflowError(); err == nil || !strings.Contains(err.Error(), "is not enabled") {
		t.Fatalf("workflow error = %v, want the role to be reported as not enabled", err)
	}
	if w.inference != 0 {
		t.Error("a disabled role reached the model")
	}
}

// The run stops for a person. Decisions for the wrong stop are ignored; the
// right one lets the publish go ahead exactly once.
func TestWorkflowWaitsForAPerson(t *testing.T) {
	w := &world{script: []any{toolCall("publish"), toolCall("search"), done("# Published\nx")}}
	env := newEnv(t, w, true)

	env.RegisterDelayedCallback(func() {
		if w.publishes != 0 {
			t.Error("published before any decision")
		}
		val, err := env.QueryWorkflow(StatusQuery)
		if err != nil {
			t.Fatal(err)
		}
		var res Result
		if err := val.Get(&res); err != nil {
			t.Fatal(err)
		}
		if res.Status != runtime.AwaitingApproval || res.PendingTool != "publish" || res.ApprovalSeq != 1 {
			t.Errorf("status query = %+v, want waiting on publish at stop 1", res)
		}
		env.SignalWorkflow(DecisionSignal, Decision{Seq: 7, Approve: true}) // not this stop
	}, time.Hour)
	env.RegisterDelayedCallback(func() {
		if w.publishes != 0 {
			t.Error("a decision for another stop was applied")
		}
		env.SignalWorkflow(DecisionSignal, Decision{Seq: 1, Approve: true})
		env.SignalWorkflow(DecisionSignal, Decision{Seq: 1, Approve: true}) // sent twice
	}, 2*time.Hour)

	env.ExecuteWorkflow(Task, task)
	res := finished(t, env)
	if res.Status != runtime.Succeeded || w.publishes != 1 {
		t.Fatalf("result = %+v with %d publishes; want succeeded with exactly 1", res, w.publishes)
	}
}

func TestWorkflowRejection(t *testing.T) {
	w := &world{script: []any{toolCall("publish"), toolCall("search"), done("# Kept as a draft\nx")}}
	env := newEnv(t, w, true)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(DecisionSignal, Decision{Seq: 1, Approve: false, Reason: "not this week"})
	}, time.Hour)

	env.ExecuteWorkflow(Task, task)
	res := finished(t, env)
	if res.Status != runtime.Succeeded || w.publishes != 0 {
		t.Fatalf("result = %+v with %d publishes; want succeeded with none", res, w.publishes)
	}
}

// Decide is retried by Temporal like any activity. A retry after the
// decision was already applied must not apply it again.
func TestDecideIsSafeToRetry(t *testing.T) {
	w := &world{script: []any{toolCall("publish"), toolCall("publish"), done("x")}}
	a := &Activities{Runtime: newRuntime(t, w, true)}
	ctx := context.Background()

	res, err := a.RunTask(ctx, task)
	if err != nil || res.ApprovalSeq != 1 {
		t.Fatalf("RunTask = %+v, %v", res, err)
	}
	approve := Decision{Seq: 1, Approve: true}
	res, err = a.Decide(ctx, task, approve)
	if err != nil || w.publishes != 1 || res.Status != runtime.AwaitingApproval || res.ApprovalSeq != 2 {
		t.Fatalf("first Decide = %+v, %v, %d publishes; want waiting at stop 2 after 1 publish", res, err, w.publishes)
	}
	// The same decision again, as a retry would deliver it. The run is now
	// waiting on a second publish, which this decision does not cover.
	res, err = a.Decide(ctx, task, approve)
	if err != nil || w.publishes != 1 || res.ApprovalSeq != 2 {
		t.Fatalf("retried Decide = %+v, %v, %d publishes; want nothing to change", res, err, w.publishes)
	}
}
