package store

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/RoundtableCI/MerlinCI/internal/runtime"
)

// TaskRecord is a task as it was requested, with where its run stands.
type TaskRecord struct {
	ID        string         `json:"id"`
	Role      string         `json:"role"`
	Input     map[string]any `json:"input"`
	CreatedBy string         `json:"createdBy"`
	CreatedAt time.Time      `json:"createdAt"`
	// Status is "pending" until the run writes its first checkpoint.
	Status string `json:"status"`
}

// Tasks stores requested tasks.
type Tasks struct{ s *Store }

// Tasks returns the task store.
func (s *Store) Tasks() *Tasks { return &Tasks{s} }

// Insert records a new task.
func (t *Tasks) Insert(ctx context.Context, task runtime.Task, createdBy string) error {
	input, err := json.Marshal(task.Input)
	if err != nil {
		return err
	}
	_, err = t.s.pool.Exec(ctx,
		"INSERT INTO tasks (id, role, input, created_by) VALUES ($1, $2, $3, $4)",
		task.ID, task.Role, input, createdBy)
	return err
}

const taskSelect = `
	SELECT t.id, t.role, t.input, t.created_by, t.created_at, COALESCE(r.status, 'pending')
	FROM tasks t LEFT JOIN run_states r ON r.task_id = t.id`

// Get returns one task.
func (t *Tasks) Get(ctx context.Context, id string) (TaskRecord, bool, error) {
	var rec TaskRecord
	var input []byte
	err := t.s.pool.QueryRow(ctx, taskSelect+" WHERE t.id = $1", id).
		Scan(&rec.ID, &rec.Role, &input, &rec.CreatedBy, &rec.CreatedAt, &rec.Status)
	if notFound(err) {
		return TaskRecord{}, false, nil
	}
	if err != nil {
		return TaskRecord{}, false, err
	}
	return rec, true, json.Unmarshal(input, &rec.Input)
}

// List returns the most recent tasks, newest first.
func (t *Tasks) List(ctx context.Context, limit int) ([]TaskRecord, error) {
	rows, err := t.s.pool.Query(ctx, taskSelect+" ORDER BY t.created_at DESC, t.id LIMIT $1", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TaskRecord{}
	for rows.Next() {
		var rec TaskRecord
		var input []byte
		if err := rows.Scan(&rec.ID, &rec.Role, &input, &rec.CreatedBy, &rec.CreatedAt, &rec.Status); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(input, &rec.Input); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// EventRecord is one stored event.
type EventRecord struct {
	ID     int64          `json:"id"`
	TaskID string         `json:"taskId"`
	Role   string         `json:"role"`
	Step   int            `json:"step"`
	Type   string         `json:"type"`
	Data   map[string]any `json:"data"`
	At     time.Time      `json:"at"`
}

// Events implements runtime.EventSink and reads events back.
type Events struct{ s *Store }

// Events returns the event log.
func (s *Store) Events() *Events { return &Events{s} }

// Record appends an event.
func (e *Events) Record(ctx context.Context, ev runtime.Event) error {
	data, err := json.Marshal(ev.Data)
	if err != nil {
		return err
	}
	if ev.Data == nil {
		data = []byte("{}")
	}
	_, err = e.s.pool.Exec(ctx,
		"INSERT INTO events (task_id, role, step, type, data) VALUES ($1, $2, $3, $4, $5)",
		ev.TaskID, ev.Role, ev.Step, ev.Type, data)
	return err
}

// List returns the events of a task in the order they happened.
func (e *Events) List(ctx context.Context, taskID string) ([]EventRecord, error) {
	rows, err := e.s.pool.Query(ctx,
		"SELECT id, task_id, role, step, type, data, at FROM events WHERE task_id = $1 ORDER BY id", taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EventRecord{}
	for rows.Next() {
		var rec EventRecord
		var data []byte
		if err := rows.Scan(&rec.ID, &rec.TaskID, &rec.Role, &rec.Step, &rec.Type, &data, &rec.At); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &rec.Data); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// Controls are the switches a person can flip while MerlinCI runs.
type Controls struct{ s *Store }

// Controls returns the control switches.
func (s *Store) Controls() *Controls { return &Controls{s} }

const pausedKey = "paused"

// Paused reports whether a person has paused MerlinCI.
func (c *Controls) Paused(ctx context.Context) (bool, error) {
	var value string
	err := c.s.pool.QueryRow(ctx, "SELECT value FROM controls WHERE key = $1", pausedKey).Scan(&value)
	if notFound(err) {
		return false, nil
	}
	return value == "true", err
}

// SetPaused pauses or resumes MerlinCI.
func (c *Controls) SetPaused(ctx context.Context, paused bool, by string) error {
	value := "false"
	if paused {
		value = "true"
	}
	_, err := c.s.pool.Exec(ctx, `
		INSERT INTO controls (key, value, updated_by, updated_at) VALUES ($1, $2, $3, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now()`,
		pausedKey, value, by)
	return err
}

// Counter implements policy.Counter from the tool-call record.
type Counter struct {
	s   *Store
	now func() time.Time
}

// Counter returns the daily action counter. now may be nil.
func (s *Store) Counter(now func() time.Time) *Counter {
	if now == nil {
		now = time.Now
	}
	return &Counter{s: s, now: now}
}

// CountToday returns how many times an action has run on a channel since
// the start of the day (UTC). Calls that were started count even when their
// outcome is unknown. If the count cannot be read it reports the highest
// possible number, so a frequency limit fails closed.
func (c *Counter) CountToday(action, channel string) int {
	now := c.now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var n int
	err := c.s.pool.QueryRow(ctx,
		"SELECT count(*) FROM effects WHERE action = $1 AND channel = $2 AND started_at >= $3",
		action, channel, start).Scan(&n)
	if err != nil {
		return math.MaxInt
	}
	return n
}
