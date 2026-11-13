package workflow

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	wf "go.temporal.io/sdk/workflow"

	"github.com/RoundtableCI/MerlinCI/internal/runtime"
)

// CampaignQuery returns the latest CampaignResult of a campaign.
const CampaignQuery = "campaign"

// Step is one step of a campaign: a task for a role, or a gate that an
// evaluator must pass.
type Step struct {
	ID        string   `json:"id"`
	Role      string   `json:"role,omitempty"`
	Evaluator string   `json:"evaluator,omitempty"`
	Needs     []string `json:"needs,omitempty"`
}

// CampaignInput starts a campaign. Steps are in an order that satisfies
// every step's needs.
type CampaignInput struct {
	ID       string         `json:"id"`
	Workflow string         `json:"workflow"`
	Steps    []Step         `json:"steps"`
	Input    map[string]any `json:"input"`
}

// Step and campaign statuses.
const (
	StepPending   = "pending"
	StepRunning   = "running"
	StepSucceeded = "succeeded"
	StepFailed    = "failed"
)

// StepResult is where one step stands.
type StepResult struct {
	ID         string `json:"id"`
	TaskID     string `json:"taskId,omitempty"`
	Status     string `json:"status"`
	ArtifactID string `json:"artifactId,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// CampaignResult is where a campaign stands.
type CampaignResult struct {
	ID       string       `json:"id"`
	Workflow string       `json:"workflow"`
	Status   string       `json:"status"`
	Reason   string       `json:"reason,omitempty"`
	Steps    []StepResult `json:"steps"`
}

// GateInput asks an evaluator to judge a stored artifact.
type GateInput struct {
	Evaluator  string `json:"evaluator"`
	ArtifactID string `json:"artifactId"`
}

// GateResult is the evaluator's answer.
type GateResult struct {
	Pass     bool   `json:"pass"`
	Feedback string `json:"feedback,omitempty"`
}

// CampaignStore keeps where campaigns stand. PostgreSQL implements it.
type CampaignStore interface {
	Save(ctx context.Context, r CampaignResult) error
}

// TaskRegistrar records a task a campaign creates, so that it can be
// inspected and decided on like any other task.
type TaskRegistrar interface {
	Register(ctx context.Context, task runtime.Task, createdBy string) error
}

// Gate runs an evaluator on a stored artifact.
func (a *Activities) Gate(ctx context.Context, in GateInput) (GateResult, error) {
	verdict, err := a.Runtime.Evaluate(ctx, in.Evaluator, in.ArtifactID)
	if err != nil {
		return GateResult{}, activityError(err)
	}
	return GateResult{Pass: verdict.Pass, Feedback: verdict.Feedback}, nil
}

// RegisterTask records a task the campaign is about to run.
func (a *Activities) RegisterTask(ctx context.Context, task runtime.Task) error {
	if a.Tasks == nil {
		return nil
	}
	return a.Tasks.Register(ctx, task, "orchestrator")
}

// SaveCampaign records where a campaign stands.
func (a *Activities) SaveCampaign(ctx context.Context, r CampaignResult) error {
	if a.Campaigns == nil {
		return nil
	}
	return a.Campaigns.Save(ctx, r)
}

// Campaign runs a workflow template from first step to last (deck slides 9
// and 14). Each role step is a task of its own, with its own checkpoints,
// budget cap and approvals; each step builds on the artifacts of the steps
// it needs. A step that fails, or a gate that rejects, stops the campaign:
// nothing downstream runs.
func Campaign(ctx wf.Context, in CampaignInput) (CampaignResult, error) {
	res := CampaignResult{ID: in.ID, Workflow: in.Workflow, Status: StepRunning}
	for _, s := range in.Steps {
		res.Steps = append(res.Steps, StepResult{ID: s.ID, Status: StepPending})
	}
	if err := wf.SetQueryHandler(ctx, CampaignQuery, func() (CampaignResult, error) { return res, nil }); err != nil {
		return res, err
	}

	var a *Activities
	bookkeeping := wf.WithActivityOptions(ctx, wf.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 2 * time.Second, BackoffCoefficient: 2, MaximumInterval: time.Minute},
	})
	gating := wf.WithActivityOptions(ctx, wf.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 5 * time.Second, BackoffCoefficient: 2, MaximumInterval: 5 * time.Minute, MaximumAttempts: 8},
	})
	save := func() error { return wf.ExecuteActivity(bookkeeping, a.SaveCampaign, res).Get(ctx, nil) }

	artifact := map[string]string{} // step ID -> the artifact it yields
	for i, step := range in.Steps {
		out := &res.Steps[i]
		out.Status = StepRunning

		var derived []string
		for _, need := range step.Needs {
			if id := artifact[need]; id != "" {
				derived = append(derived, id)
			}
		}

		var reason string
		if step.Role != "" {
			task := runtime.Task{ID: in.ID + "." + step.ID, Role: step.Role, Input: in.Input, DerivedFrom: derived}
			out.TaskID = task.ID
			if err := wf.ExecuteActivity(bookkeeping, a.RegisterTask, task).Get(ctx, nil); err != nil {
				return res, err
			}
			if err := save(); err != nil {
				return res, err
			}
			child := wf.WithChildOptions(ctx, wf.ChildWorkflowOptions{WorkflowID: task.ID})
			var r Result
			if err := wf.ExecuteChildWorkflow(child, Task, task).Get(ctx, &r); err != nil {
				reason = "the task could not be completed: " + err.Error()
			} else if r.Status != runtime.Succeeded {
				reason = r.Reason
				if reason == "" {
					reason = "the task ended as " + string(r.Status)
				}
			} else {
				out.ArtifactID = r.ArtifactID
			}
		} else {
			if len(derived) == 0 {
				reason = "there is no artifact for " + step.Evaluator + " to check"
			} else {
				// A gate checks the artifact it is given and passes
				// the same artifact on.
				out.ArtifactID = derived[0]
				var g GateResult
				if err := wf.ExecuteActivity(gating, a.Gate, GateInput{Evaluator: step.Evaluator, ArtifactID: derived[0]}).Get(ctx, &g); err != nil {
					reason = "the check could not be completed: " + err.Error()
				} else if !g.Pass {
					reason = fmt.Sprintf("%s rejected it: %s", step.Evaluator, g.Feedback)
				}
			}
		}

		if reason != "" {
			out.Status, out.Reason = StepFailed, reason
			res.Status, res.Reason = StepFailed, "step "+step.ID+": "+reason
			return res, save()
		}
		out.Status = StepSucceeded
		artifact[step.ID] = out.ArtifactID
		if err := save(); err != nil {
			return res, err
		}
	}
	res.Status = StepSucceeded
	return res, save()
}
