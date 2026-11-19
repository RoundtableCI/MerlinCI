package evaluator

import (
	"context"

	"github.com/RoundtableCI/MerlinCI/pkg/tool"
)

// Facts is what an output may rely on: the approved product profile.
type Facts struct {
	Features         []string
	ApprovedClaims   []string
	ProhibitedClaims []string
}

// Input is an output awaiting evaluation.
type Input struct {
	ArtifactKind string
	Content      string
	Sources      []tool.Source
	Facts        Facts
	// CostCapMicros is the most the evaluation may cost, in millionths of
	// the budget currency. Zero means the task sets no cap.
	CostCapMicros int64
}

// Verdict is the result of an evaluation. Feedback goes back to the agent
// when the verdict is a failure.
type Verdict struct {
	Pass     bool
	Feedback string
	// CostMicros is what the evaluation cost, in millionths of the budget
	// currency. It is zero for an evaluator that calls no model.
	CostMicros int64
}

// Evaluator checks an output before it is stored as an artifact.
type Evaluator interface {
	Name() string
	Evaluate(ctx context.Context, in Input) (Verdict, error)
}
