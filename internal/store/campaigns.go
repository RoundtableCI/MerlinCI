package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/RoundtableCI/MerlinCI/internal/runtime"
	"github.com/RoundtableCI/MerlinCI/internal/workflow"
)

// CampaignRecord is a campaign as it was requested, with where it stands.
type CampaignRecord struct {
	ID        string                  `json:"id"`
	Workflow  string                  `json:"workflow"`
	Input     map[string]any          `json:"input"`
	CreatedBy string                  `json:"createdBy"`
	CreatedAt time.Time               `json:"createdAt"`
	UpdatedAt time.Time               `json:"updatedAt"`
	State     workflow.CampaignResult `json:"state"`
}

// Campaigns stores campaigns. It implements workflow.CampaignStore.
type Campaigns struct{ s *Store }

// Campaigns returns the campaign store.
func (s *Store) Campaigns() *Campaigns { return &Campaigns{s} }

// Insert records a new campaign in its starting state.
func (c *Campaigns) Insert(ctx context.Context, in workflow.CampaignInput, createdBy string) error {
	input, err := json.Marshal(in.Input)
	if err != nil {
		return err
	}
	start := workflow.CampaignResult{ID: in.ID, Workflow: in.Workflow, Status: workflow.StepRunning}
	for _, s := range in.Steps {
		start.Steps = append(start.Steps, workflow.StepResult{ID: s.ID, Status: workflow.StepPending})
	}
	state, err := json.Marshal(start)
	if err != nil {
		return err
	}
	_, err = c.s.pool.Exec(ctx,
		"INSERT INTO campaigns (id, workflow, input, status, state, created_by) VALUES ($1, $2, $3, $4, $5, $6)",
		in.ID, in.Workflow, input, start.Status, state, createdBy)
	return err
}

// Save records where a campaign stands.
func (c *Campaigns) Save(ctx context.Context, r workflow.CampaignResult) error {
	state, err := json.Marshal(r)
	if err != nil {
		return err
	}
	tag, err := c.s.pool.Exec(ctx,
		"UPDATE campaigns SET status = $2, state = $3, updated_at = now() WHERE id = $1", r.ID, r.Status, state)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("campaign %s is not recorded", r.ID)
	}
	return nil
}

const campaignSelect = "SELECT id, workflow, input, created_by, created_at, updated_at, state FROM campaigns"

func scanCampaign(row interface{ Scan(...any) error }) (CampaignRecord, error) {
	var rec CampaignRecord
	var input, state []byte
	if err := row.Scan(&rec.ID, &rec.Workflow, &input, &rec.CreatedBy, &rec.CreatedAt, &rec.UpdatedAt, &state); err != nil {
		return rec, err
	}
	if err := json.Unmarshal(input, &rec.Input); err != nil {
		return rec, err
	}
	return rec, json.Unmarshal(state, &rec.State)
}

// Get returns one campaign.
func (c *Campaigns) Get(ctx context.Context, id string) (CampaignRecord, bool, error) {
	rec, err := scanCampaign(c.s.pool.QueryRow(ctx, campaignSelect+" WHERE id = $1", id))
	if notFound(err) {
		return CampaignRecord{}, false, nil
	}
	return rec, err == nil, err
}

// List returns the most recent campaigns, newest first.
func (c *Campaigns) List(ctx context.Context, limit int) ([]CampaignRecord, error) {
	rows, err := c.s.pool.Query(ctx, campaignSelect+" ORDER BY created_at DESC, id LIMIT $1", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CampaignRecord{}
	for rows.Next() {
		rec, err := scanCampaign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// Register records a task created by a campaign. Recording the same task
// again changes nothing. It implements workflow.TaskRegistrar.
func (t *Tasks) Register(ctx context.Context, task runtime.Task, createdBy string) error {
	input, err := json.Marshal(task.Input)
	if err != nil {
		return err
	}
	_, err = t.s.pool.Exec(ctx,
		"INSERT INTO tasks (id, role, input, created_by) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING",
		task.ID, task.Role, input, createdBy)
	return err
}
