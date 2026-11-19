package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/RoundtableCI/MerlinCI/internal/runtime"
)

// TestOnATemporalServer runs the workflow on a real Temporal server rather
// than the in-process test environment. Set MERLIN_TEST_TEMPORAL_ADDRESS to
// run it, for example against deploy/docker-compose.yml:
//
//	MERLIN_TEST_TEMPORAL_ADDRESS=localhost:7233
//
// The run stops for a person, ignores a decision for the wrong stop, goes
// ahead on the right one, hits a failure that Temporal retries, and
// finishes having published exactly once.
func TestOnATemporalServer(t *testing.T) {
	address := os.Getenv("MERLIN_TEST_TEMPORAL_ADDRESS")
	if address == "" {
		t.Skip("MERLIN_TEST_TEMPORAL_ADDRESS is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	c, err := client.Dial(client.Options{HostPort: address})
	if err != nil {
		t.Fatalf("connect to Temporal: %v", err)
	}
	defer c.Close()

	w := &world{script: []any{
		toolCall("publish"),            // stops for a person
		errors.New("connection reset"), // fails after the publish; Temporal retries
		toolCall("search"),
		done("# Published\nx"),
	}}
	unique := fmt.Sprintf("merlin-test-%d", time.Now().UnixNano())
	wk := worker.New(c, unique, worker.Options{})
	Register(wk, &Activities{Runtime: newRuntime(t, w, true)})
	if err := wk.Start(); err != nil {
		t.Fatalf("start worker: %v", err)
	}
	defer wk.Stop()

	tk := runtime.Task{ID: unique, Role: "researcher", Input: map[string]any{"topic": "model comparison"}}
	run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: unique, TaskQueue: unique}, Task, tk)
	if err != nil {
		t.Fatalf("start workflow: %v", err)
	}

	// Wait until the run reports that it is waiting for a person.
	var status Result
	for status.Status != runtime.AwaitingApproval {
		if ctx.Err() != nil {
			t.Fatalf("the run never waited for approval; last status %+v", status)
		}
		time.Sleep(200 * time.Millisecond)
		val, err := c.QueryWorkflow(ctx, unique, run.GetRunID(), StatusQuery)
		if err != nil {
			continue // the workflow has not handled its first task yet
		}
		if err := val.Get(&status); err != nil {
			t.Fatal(err)
		}
	}
	if status.PendingTool != "publish" || status.ApprovalSeq != 1 || w.publishes != 0 {
		t.Fatalf("status = %+v with %d publishes; want waiting on publish at stop 1 with none", status, w.publishes)
	}

	if err := c.SignalWorkflow(ctx, unique, run.GetRunID(), DecisionSignal, Decision{Seq: 9, Approve: true}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	if w.publishes != 0 {
		t.Fatal("a decision for another stop was applied")
	}
	if err := c.SignalWorkflow(ctx, unique, run.GetRunID(), DecisionSignal, Decision{Seq: 1, Approve: true}); err != nil {
		t.Fatal(err)
	}

	var res Result
	if err := run.Get(ctx, &res); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	if res.Status != runtime.Succeeded || res.ArtifactID == "" {
		t.Fatalf("result = %+v, want succeeded with an artifact", res)
	}
	if w.publishes != 1 {
		t.Errorf("published %d times across the retry, want exactly 1", w.publishes)
	}
	if w.inference != 4 {
		t.Errorf("inference calls = %d, want 4 (one of them the failure)", w.inference)
	}
}
