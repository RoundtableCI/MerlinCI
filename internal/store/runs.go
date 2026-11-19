package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/RoundtableCI/MerlinCI/internal/artifacts"
	"github.com/RoundtableCI/MerlinCI/internal/runtime"
	"github.com/RoundtableCI/MerlinCI/pkg/tool"
)

// Checkpoints implements runtime.Checkpointer.
type Checkpoints struct{ s *Store }

// Checkpoints returns the run checkpoint store.
func (s *Store) Checkpoints() *Checkpoints { return &Checkpoints{s} }

func (c *Checkpoints) Save(ctx context.Context, state runtime.State) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	// The checkpoint of a cancelled run can only be replaced by another
	// cancelled state. A worker still running a cancelled task finds its
	// next checkpoint refused, and stops.
	tag, err := c.s.pool.Exec(ctx, `
		INSERT INTO run_states (task_id, role, status, state, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (task_id) DO UPDATE
		SET role = EXCLUDED.role, status = EXCLUDED.status, state = EXCLUDED.state, updated_at = now()
		WHERE run_states.status <> 'cancelled' OR EXCLUDED.status = 'cancelled'`,
		state.TaskID, state.Role, string(state.Status), raw)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return runtime.ErrCancelled
	}
	return nil
}

func (c *Checkpoints) Load(ctx context.Context, taskID string) (runtime.State, bool, error) {
	var raw []byte
	err := c.s.pool.QueryRow(ctx, "SELECT state FROM run_states WHERE task_id = $1", taskID).Scan(&raw)
	if notFound(err) {
		return runtime.State{}, false, nil
	}
	if err != nil {
		return runtime.State{}, false, err
	}
	var state runtime.State
	if err := json.Unmarshal(raw, &state); err != nil {
		return runtime.State{}, false, fmt.Errorf("checkpoint of task %s is unreadable: %w", taskID, err)
	}
	return state, true, nil
}

// Effects implements runtime.Effects.
type Effects struct{ s *Store }

// Effects returns the tool-call record.
func (s *Store) Effects() *Effects { return &Effects{s} }

func (e *Effects) Get(ctx context.Context, key string) (*runtime.Effect, error) {
	out := runtime.Effect{Key: key}
	var status string
	var result *string
	var sources []byte
	err := e.s.pool.QueryRow(ctx,
		"SELECT task_id, tool, action, status, result, sources FROM effects WHERE key = $1", key).
		Scan(&out.TaskID, &out.Tool, &out.Action, &status, &result, &sources)
	if notFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out.Done = status == "done"
	if result != nil {
		out.Result = *result
	}
	if len(sources) > 0 {
		if err := json.Unmarshal(sources, &out.Sources); err != nil {
			return nil, fmt.Errorf("sources of tool call %s are unreadable: %w", key, err)
		}
	}
	return &out, nil
}

func (e *Effects) Begin(ctx context.Context, effect runtime.Effect) error {
	_, err := e.s.pool.Exec(ctx, `
		INSERT INTO effects (key, task_id, tool, action, channel, status)
		VALUES ($1, $2, $3, $4, $5, 'started')
		ON CONFLICT (key) DO NOTHING`,
		effect.Key, effect.TaskID, effect.Tool, effect.Action, effect.Channel)
	return err
}

func (e *Effects) Finish(ctx context.Context, key, result string, sources []tool.Source) error {
	raw, err := json.Marshal(sources)
	if err != nil {
		return err
	}
	tag, err := e.s.pool.Exec(ctx, `
		UPDATE effects SET status = 'done', result = $2, sources = $3, finished_at = now()
		WHERE key = $1`, key, result, raw)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("tool call %s was never recorded as started", key)
	}
	return nil
}

// Artifacts implements artifacts.Index.
type Artifacts struct{ s *Store }

// Artifacts returns the artifact record index.
func (s *Store) Artifacts() *Artifacts { return &Artifacts{s} }

func (a *Artifacts) Insert(ctx context.Context, rec artifacts.Artifact) error {
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = a.s.pool.Exec(ctx, `
		INSERT INTO artifacts (id, kind, task_id, role, record, created_at)
		VALUES ($1, $2, $3, $4, $5, $6::timestamptz)`,
		rec.ID, rec.Kind, rec.TaskID, rec.Role, raw, rec.CreatedAt)
	return err
}

func (a *Artifacts) Get(ctx context.Context, id string) (artifacts.Artifact, bool, error) {
	var raw []byte
	err := a.s.pool.QueryRow(ctx, "SELECT record FROM artifacts WHERE id = $1", id).Scan(&raw)
	if notFound(err) {
		return artifacts.Artifact{}, false, nil
	}
	if err != nil {
		return artifacts.Artifact{}, false, err
	}
	var rec artifacts.Artifact
	if err := json.Unmarshal(raw, &rec); err != nil {
		return artifacts.Artifact{}, false, fmt.Errorf("artifact %s is unreadable: %w", id, err)
	}
	return rec, true, nil
}
