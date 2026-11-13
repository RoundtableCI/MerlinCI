package workflow

import (
	"context"
	"errors"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	wf "go.temporal.io/sdk/workflow"

	"github.com/RoundtableCI/MerlinCI/internal/runtime"
)

const (
	// TaskQueue is the default Temporal task queue.
	TaskQueue = "merlin"
	// DecisionSignal carries a person's Decision to a waiting run.
	DecisionSignal = "decision"
	// StatusQuery returns the latest Result of a run.
	StatusQuery = "status"
)

// Result is what a run reports to the workflow. It is deliberately small:
// the conversation and the evidence stay in PostgreSQL, not in workflow
// history.
type Result struct {
	TaskID     string         `json:"taskId"`
	Status     runtime.Status `json:"status"`
	Reason     string         `json:"reason,omitempty"`
	ArtifactID string         `json:"artifactId,omitempty"`
	Spent      string         `json:"spent"`
	// PendingTool and ApprovalSeq describe what the run is waiting on.
	PendingTool string `json:"pendingTool,omitempty"`
	ApprovalSeq int    `json:"approvalSeq,omitempty"`
}

// Decision is a person's answer to a run that is waiting for approval.
type Decision struct {
	// Seq is the ApprovalSeq of the stop this decision answers. A decision
	// for any other stop is ignored.
	Seq     int    `json:"seq"`
	Approve bool   `json:"approve"`
	Reason  string `json:"reason,omitempty"`
}

// Activities are the steps the workflow runs. They do the real work, so
// they may be retried; the runtime's checkpoints and tool-call records make
// a retry continue the run instead of repeating it.
type Activities struct {
	Runtime *runtime.Runtime
	// Tasks and Campaigns record what campaigns do. Both may be nil.
	Tasks     TaskRegistrar
	Campaigns CampaignStore
}

// Register adds the workflow and its activities to a worker.
func Register(w worker.Registry, a *Activities) {
	w.RegisterWorkflow(Task)
	w.RegisterWorkflow(Campaign)
	w.RegisterActivity(a)
}

// RunTask runs a task to its next stopping point, resuming from the last
// checkpoint when the task has run before.
func (a *Activities) RunTask(ctx context.Context, task runtime.Task) (Result, error) {
	state, err := a.Runtime.Run(ctx, task)
	return outcome(state, err)
}

// Decide applies a person's decision and continues the run. It is safe to
// retry: a decision is applied only while the run is waiting at the stop
// the decision names.
func (a *Activities) Decide(ctx context.Context, task runtime.Task, d Decision) (Result, error) {
	// Run does nothing to a waiting run, and resumes one that an earlier
	// attempt of this activity left in progress.
	state, err := a.Runtime.Run(ctx, task)
	if err != nil {
		return outcome(state, err)
	}
	if state.Status != runtime.AwaitingApproval || state.ApprovalSeq != d.Seq {
		return result(state), nil
	}
	if d.Approve {
		state, err = a.Runtime.Approve(ctx, task.ID)
	} else {
		state, err = a.Runtime.Reject(ctx, task.ID, d.Reason)
	}
	return outcome(state, err)
}

// StatusPaused is reported while a person has MerlinCI paused. It is not a
// failure: the workflow waits and tries again.
const StatusPaused runtime.Status = "paused"

// PausePoll is how often a paused workflow checks whether it may continue.
const PausePoll = time.Minute

func outcome(state runtime.State, err error) (Result, error) {
	if errors.Is(err, runtime.ErrCancelled) {
		// A person cancelled the run. That is its final state, not a
		// failure to retry.
		res := result(state)
		res.TaskID, res.Status, res.Reason = state.TaskID, runtime.Cancelled, "cancelled by a person"
		return res, nil
	}
	if errors.Is(err, runtime.ErrPaused) {
		res := result(state)
		res.TaskID, res.Status = state.TaskID, StatusPaused
		return res, nil
	}
	return result(state), activityError(err)
}

func result(s runtime.State) Result {
	r := Result{TaskID: s.TaskID, Status: s.Status, Reason: s.Reason, ArtifactID: s.ArtifactID, Spent: s.Spent.String(), ApprovalSeq: s.ApprovalSeq}
	if s.PendingCall != nil {
		r.PendingTool = s.PendingCall.Name
	}
	return r
}

// activityError marks errors that retrying cannot fix, so Temporal does not
// retry them.
func activityError(err error) error {
	if errors.Is(err, runtime.ErrNotRunnable) {
		return temporal.NewNonRetryableApplicationError(err.Error(), "NotRunnable", err)
	}
	return err
}

// Task is the durable workflow for one task: run it, and each time it stops
// for a person, wait for that person's decision and continue. Temporal
// keeps the workflow alive across worker crashes and restarts.
func Task(ctx wf.Context, task runtime.Task) (Result, error) {
	ctx = wf.WithActivityOptions(ctx, wf.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    8,
		},
	})

	var res Result
	if err := wf.SetQueryHandler(ctx, StatusQuery, func() (Result, error) { return res, nil }); err != nil {
		return res, err
	}

	var a *Activities
	if err := wf.ExecuteActivity(ctx, a.RunTask, task).Get(ctx, &res); err != nil {
		return res, err
	}
	decisions := wf.GetSignalChannel(ctx, DecisionSignal)
	// held is a decision that arrived while MerlinCI was paused. It is
	// applied once MerlinCI is resumed, not dropped.
	var held *Decision
	for {
		switch res.Status {
		case StatusPaused:
			if err := wf.Sleep(ctx, PausePoll); err != nil {
				return res, err
			}
			var err error
			if held != nil {
				err = wf.ExecuteActivity(ctx, a.Decide, task, *held).Get(ctx, &res)
			} else {
				err = wf.ExecuteActivity(ctx, a.RunTask, task).Get(ctx, &res)
			}
			if err != nil {
				return res, err
			}
		case runtime.AwaitingApproval:
			held = nil
			var d Decision
			decisions.Receive(ctx, &d)
			if d.Seq != res.ApprovalSeq {
				continue // a decision for a stop the run is no longer at
			}
			held = &d
			if err := wf.ExecuteActivity(ctx, a.Decide, task, d).Get(ctx, &res); err != nil {
				return res, err
			}
		default:
			return res, nil
		}
	}
}
