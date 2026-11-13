package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/RoundtableCI/MerlinCI/internal/tools"
	"github.com/RoundtableCI/MerlinCI/pkg/tool"
)

// failingCheckpoints fails one chosen save, as if the process died there.
type failingCheckpoints struct {
	*MemoryCheckpoints
	failOn int
	n      int
}

func (f *failingCheckpoints) Save(ctx context.Context, s State) error {
	f.n++
	if f.n == f.failOn {
		return errors.New("process died")
	}
	return f.MemoryCheckpoints.Save(ctx, s)
}

// A tool finished, but the process died before the checkpoint that would
// have remembered it. On resume the recorded outcome is used; the tool does
// not run a second time.
func TestFinishedToolCallIsNotRepeated(t *testing.T) {
	h := newHarness(t, nil, calls("search"), final("# An opportunity\nx"))
	// Save 1 follows the first inference; save 2 follows the tool call.
	h.rt.Checkpoints = &failingCheckpoints{MemoryCheckpoints: h.cps, failOn: 2}

	if _, err := h.rt.Run(context.Background(), task); err == nil {
		t.Fatal("Run returned no error although a checkpoint failed")
	}
	if h.search.calls != 1 {
		t.Fatalf("search ran %d times before the interruption, want 1", h.search.calls)
	}

	state := mustRun(t, h)
	if state.Status != Succeeded {
		t.Fatalf("status after resume = %s (%s)", state.Status, state.Reason)
	}
	if h.search.calls != 1 {
		t.Errorf("search ran %d times in total, want 1: a recorded call must not be repeated", h.search.calls)
	}
	if len(state.Sources) != 1 {
		t.Errorf("sources = %d, want the 1 source recorded with the call", len(state.Sources))
	}
}

// panicTool dies in the middle of its first call, like a process killed
// while a publish request is in flight.
type panicTool struct {
	fakeTool
	died bool
}

func (p *panicTool) Call(ctx context.Context, in json.RawMessage) (tool.Result, error) {
	if !p.died {
		p.died = true
		p.calls++
		panic("process killed mid-publish")
	}
	return p.fakeTool.Call(ctx, in)
}

// Publishing actions are never repeated on recovery (deck slide 10). When a
// publish was interrupted and nobody knows whether it went out, the run
// waits for a person instead of publishing again, even though the original
// approval had been given.
func TestInterruptedPublishIsNotRepeated(t *testing.T) {
	h := newHarness(t, nil, calls("publish"), calls("search"), final("# Published\nx"))
	ctx := context.Background()

	// Swap in a publish tool that dies mid-call.
	dying := &panicTool{fakeTool: fakeTool{name: "publish", action: "publish.post"}}
	h.rt.Tools = registryWith(t, h.search, dying, h.spend)

	mustRun(t, h) // stops, waiting for approval
	func() {
		defer func() { _ = recover() }()
		_, _ = h.rt.Approve(ctx, "t1")
	}()
	if dying.calls != 1 {
		t.Fatalf("publish was attempted %d times, want 1", dying.calls)
	}

	// The process restarts and the task runs again.
	state := mustRun(t, h)
	if state.Status != AwaitingApproval {
		t.Fatalf("status = %s (%s); want the run to wait for a person", state.Status, state.Reason)
	}
	if !strings.Contains(state.Reason, "interrupted and may already have happened") {
		t.Errorf("reason = %q, want it to say the action was interrupted", state.Reason)
	}
	if dying.calls != 1 {
		t.Fatalf("publish was attempted %d times after recovery, want still 1", dying.calls)
	}

	// A person checks that nothing went out, and approves again.
	state, err := h.rt.Approve(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != Succeeded || dying.calls != 2 {
		t.Fatalf("status %s, publish attempts %d; want succeeded after exactly one more", state.Status, dying.calls)
	}
}

// An interrupted call in the automatic tier, such as a search, is safe to
// make again and does not need a person.
func TestInterruptedAutomaticCallRunsAgain(t *testing.T) {
	h := newHarness(t, nil, calls("search"), final("# An opportunity\nx"))
	dying := &panicTool{fakeTool: fakeTool{name: "search", action: "research.search"}}
	h.rt.Tools = registryWith(t, dying, h.publish, h.spend)

	func() {
		defer func() { _ = recover() }()
		_, _ = h.rt.Run(context.Background(), task)
	}()
	state := mustRun(t, h)
	if state.Status != Succeeded || dying.calls != 2 {
		t.Fatalf("status %s (%s), search attempts %d; want succeeded after a second attempt", state.Status, state.Reason, dying.calls)
	}
}

func registryWith(t *testing.T, list ...tool.Tool) *tools.Registry {
	t.Helper()
	r := tools.NewRegistry()
	for _, tl := range list {
		if err := r.Register(tl); err != nil {
			t.Fatal(err)
		}
	}
	return r
}
