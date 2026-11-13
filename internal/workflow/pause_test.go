package workflow

import (
	"context"
	"testing"
	"time"

	"go.temporal.io/sdk/testsuite"

	"github.com/RoundtableCI/MerlinCI/internal/runtime"
)

// A person approves an action while MerlinCI is paused. The approval must
// not run the action during the pause, and must not be lost: once MerlinCI
// is resumed the action goes ahead, exactly once, without asking again.
func TestDecisionMadeWhilePausedIsAppliedOnResume(t *testing.T) {
	w := &world{script: []any{toolCall("publish"), toolCall("search"), done("# Published\nx")}}
	rt := newRuntime(t, w, true)
	paused := false
	rt.Paused = func(context.Context) (bool, error) { return paused, nil }

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivity(&Activities{Runtime: rt})

	env.RegisterDelayedCallback(func() {
		paused = true
		env.SignalWorkflow(DecisionSignal, Decision{Seq: 1, Approve: true})
	}, time.Hour)
	env.RegisterDelayedCallback(func() {
		if w.publishes != 0 {
			t.Error("the action ran while MerlinCI was paused")
		}
		val, err := env.QueryWorkflow(StatusQuery)
		if err != nil {
			t.Fatal(err)
		}
		var res Result
		if err := val.Get(&res); err != nil {
			t.Fatal(err)
		}
		if res.Status != StatusPaused {
			t.Errorf("status while paused = %s, want paused", res.Status)
		}
		paused = false
	}, 2*time.Hour)

	env.ExecuteWorkflow(Task, task)
	res := finished(t, env)
	if res.Status != runtime.Succeeded || w.publishes != 1 {
		t.Fatalf("result = %+v with %d publishes; want succeeded with exactly 1", res, w.publishes)
	}
}

// A run that is paused before it starts does nothing until it is resumed.
func TestPausedRunWaits(t *testing.T) {
	w := &world{script: []any{toolCall("search"), done("# An opportunity\nx")}}
	rt := newRuntime(t, w, true)
	paused := true
	rt.Paused = func(context.Context) (bool, error) { return paused, nil }

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivity(&Activities{Runtime: rt})
	env.RegisterDelayedCallback(func() {
		if w.inference != 0 || w.searches != 0 {
			t.Errorf("%d model calls and %d searches while paused, want none", w.inference, w.searches)
		}
		paused = false
	}, 3*time.Hour)

	env.ExecuteWorkflow(Task, task)
	if res := finished(t, env); res.Status != runtime.Succeeded {
		t.Fatalf("result = %+v", res)
	}
}
