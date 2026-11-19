package runtime

import (
	"context"
	"errors"
	"sync"

	"github.com/RoundtableCI/MerlinCI/internal/artifacts"
	"github.com/RoundtableCI/MerlinCI/internal/budget"
	"github.com/RoundtableCI/MerlinCI/pkg/provider"
	"github.com/RoundtableCI/MerlinCI/pkg/tool"
)

// Task is a unit of work for one role.
type Task struct {
	ID    string
	Role  string
	Input map[string]any
	// DerivedFrom names the artifacts this task builds on, such as the
	// brief behind an article. Their content is given to the agent and
	// their sources carry over to what it produces.
	DerivedFrom []string
}

// Status is where a run stands.
type Status string

const (
	Running          Status = "running"
	AwaitingApproval Status = "awaiting_approval"
	Succeeded        Status = "succeeded"
	Failed           Status = "failed"
	// Cancelled: a person stopped the run. Nothing can move it on.
	Cancelled Status = "cancelled"
)

// State is everything needed to continue a run. It is saved at every
// checkpoint, so a run resumes from the last one instead of starting again.
type State struct {
	TaskID   string             `json:"taskId"`
	Role     string             `json:"role"`
	Status   Status             `json:"status"`
	Reason   string             `json:"reason,omitempty"` // why the run failed or is waiting
	Step     int                `json:"step"`             // inference calls made
	Messages []provider.Message `json:"messages"`

	Spent        budget.Micros `json:"spent"`
	InputTokens  int           `json:"inputTokens"`
	OutputTokens int           `json:"outputTokens"`

	Sources     []tool.Source          `json:"sources,omitempty"`
	Evaluations []artifacts.Evaluation `json:"evaluations,omitempty"`
	DerivedFrom []string               `json:"derivedFrom,omitempty"`

	// PendingCall is the tool call waiting for a person, when Status is
	// AwaitingApproval.
	PendingCall *provider.ToolCall `json:"pendingCall,omitempty"`
	// ApprovalSeq counts how many times the run has stopped to wait for a
	// person. A decision names the stop it answers, so a decision that
	// arrives late or twice cannot approve a different stop.
	ApprovalSeq int `json:"approvalSeq,omitempty"`
	// ApprovedCallID is the one call a person has approved.
	ApprovedCallID string `json:"approvedCallId,omitempty"`

	ArtifactID   string `json:"artifactId,omitempty"`
	ConfigDigest string `json:"configDigest"`
}

// Done reports whether the run has finished.
func (s State) Done() bool {
	return s.Status == Succeeded || s.Status == Failed || s.Status == Cancelled
}

// ErrCancelled means a person cancelled the run. Its checkpoint can no
// longer be changed, so the run cannot continue.
var ErrCancelled = errors.New("the run was cancelled")

// Checkpointer saves and loads run state. PostgreSQL implements it in
// production.
//
// Save must refuse, with ErrCancelled, to replace the checkpoint of a
// cancelled run with anything but another cancelled state. This is what
// stops a cancelled run: its next checkpoint fails, and it goes no further.
type Checkpointer interface {
	Save(ctx context.Context, s State) error
	Load(ctx context.Context, taskID string) (State, bool, error)
}

// MemoryCheckpoints is a Checkpointer kept in memory, for tests and local runs.
type MemoryCheckpoints struct {
	mu     sync.Mutex
	states map[string]State
	// Saves counts checkpoints written.
	Saves int
}

// NewMemoryCheckpoints returns an empty in-memory checkpointer.
func NewMemoryCheckpoints() *MemoryCheckpoints {
	return &MemoryCheckpoints{states: map[string]State{}}
}

func (m *MemoryCheckpoints) Save(_ context.Context, s State) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if old, ok := m.states[s.TaskID]; ok && old.Status == Cancelled && s.Status != Cancelled {
		return ErrCancelled
	}
	s.Messages = append([]provider.Message(nil), s.Messages...)
	s.Sources = append([]tool.Source(nil), s.Sources...)
	s.Evaluations = append([]artifacts.Evaluation(nil), s.Evaluations...)
	m.states[s.TaskID] = s
	m.Saves++
	return nil
}

func (m *MemoryCheckpoints) Load(_ context.Context, taskID string) (State, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.states[taskID]
	return s, ok, nil
}
