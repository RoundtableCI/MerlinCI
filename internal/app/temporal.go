package app

import (
	"context"
	"fmt"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/RoundtableCI/MerlinCI/internal/runtime"
	"github.com/RoundtableCI/MerlinCI/internal/workflow"
)

// DialTemporal connects to the Temporal server.
func DialTemporal(env Env) (client.Client, error) {
	c, err := client.Dial(client.Options{HostPort: env.TemporalAddress, Namespace: env.TemporalNamespace})
	if err != nil {
		return nil, fmt.Errorf("connect to Temporal at %s: %w", env.TemporalAddress, err)
	}
	return c, nil
}

// Workflows implements api.Workflows on Temporal.
type Workflows struct {
	Client    client.Client
	TaskQueue string
}

// Start begins the durable workflow for a task. The task ID is the workflow
// ID, and Temporal refuses a second workflow with the same ID, so a task
// can never be started twice.
func (w Workflows) Start(ctx context.Context, task runtime.Task) error {
	_, err := w.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       task.ID,
		TaskQueue:                w.TaskQueue,
		WorkflowIDReusePolicy:    enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		WorkflowIDConflictPolicy: enumspb.WORKFLOW_ID_CONFLICT_POLICY_FAIL,
	}, workflow.Task, task)
	return err
}

// StartCampaign begins the durable workflow for a campaign.
func (w Workflows) StartCampaign(ctx context.Context, in workflow.CampaignInput) error {
	_, err := w.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       in.ID,
		TaskQueue:                w.TaskQueue,
		WorkflowIDReusePolicy:    enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		WorkflowIDConflictPolicy: enumspb.WORKFLOW_ID_CONFLICT_POLICY_FAIL,
	}, workflow.Campaign, in)
	return err
}

// Decide delivers a person's decision to a waiting run.
func (w Workflows) Decide(ctx context.Context, taskID string, d workflow.Decision) error {
	return w.Client.SignalWorkflow(ctx, taskID, "", workflow.DecisionSignal, d)
}

// Cancel stops a run.
func (w Workflows) Cancel(ctx context.Context, taskID string) error {
	return w.Client.CancelWorkflow(ctx, taskID, "")
}

// RunWorker runs tasks until the process is interrupted.
func RunWorker(c client.Client, core *Core) error {
	w := worker.New(c, core.Env.TaskQueue, worker.Options{})
	workflow.Register(w, core.Activities())
	return w.Run(worker.InterruptCh())
}

// Activities returns the workflow activities, wired to this core.
func (c *Core) Activities() *workflow.Activities {
	return &workflow.Activities{Runtime: c.Runtime, Tasks: c.Store.Tasks(), Campaigns: c.Store.Campaigns()}
}
