package runtime

import (
	"context"
	"sync"

	"github.com/RoundtableCI/MerlinCI/pkg/tool"
)

// Effect is the record of one tool call.
type Effect struct {
	Key    string
	TaskID string
	Tool   string
	Action string
	// Channel is the channel the call acts on, when it acts on one.
	Channel string
	// Done is false while the call is running, or when the process stopped
	// before the call's outcome was recorded.
	Done    bool
	Result  string
	Sources []tool.Source
}

// Effects records every tool call before it runs and after it finishes, so
// that a call is never made twice by accident. This is what keeps publishing
// actions from being repeated on recovery (deck slide 10).
type Effects interface {
	// Get returns the record of a call, or nil when there is none.
	Get(ctx context.Context, key string) (*Effect, error)
	// Begin records that a call is about to run. It changes nothing when
	// the call is already recorded.
	Begin(ctx context.Context, e Effect) error
	// Finish records the outcome of a call.
	Finish(ctx context.Context, key, result string, sources []tool.Source) error
}

// MemoryEffects is an Effects kept in memory, for tests and local runs.
type MemoryEffects struct {
	mu    sync.Mutex
	byKey map[string]*Effect
}

// NewMemoryEffects returns an empty in-memory record.
func NewMemoryEffects() *MemoryEffects { return &MemoryEffects{byKey: map[string]*Effect{}} }

func (m *MemoryEffects) Get(_ context.Context, key string) (*Effect, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.byKey[key]; ok {
		c := *e
		return &c, nil
	}
	return nil, nil
}

func (m *MemoryEffects) Begin(_ context.Context, e Effect) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byKey[e.Key]; !ok {
		e.Done = false
		m.byKey[e.Key] = &e
	}
	return nil
}

func (m *MemoryEffects) Finish(_ context.Context, key, result string, sources []tool.Source) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.byKey[key]; ok {
		e.Done, e.Result, e.Sources = true, result, sources
	}
	return nil
}

// Event is one thing a run did, or one decision made about it.
type Event struct {
	TaskID string
	Role   string
	Step   int
	Type   string
	Data   map[string]any
}

// EventSink keeps events permanently, so that a run can be inspected long
// after it finished (deck slide 14).
type EventSink interface {
	Record(ctx context.Context, e Event) error
}
