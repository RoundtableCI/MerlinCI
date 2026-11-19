package eval

import (
	"context"
	"strings"

	"github.com/RoundtableCI/MerlinCI/pkg/evaluator"
)

// Evidence is the first evaluator: an output must be traced to at least one
// saved source and must not make a prohibited claim. It is deliberately
// simple and calls no model. Brand Guardian, in package brandguardian, is
// the full quality gate (deck slides 9, 13).
type Evidence struct{}

// Name implements evaluator.Evaluator.
func (Evidence) Name() string { return "evidence" }

// Evaluate implements evaluator.Evaluator.
func (Evidence) Evaluate(_ context.Context, in evaluator.Input) (evaluator.Verdict, error) {
	if strings.TrimSpace(in.Content) == "" {
		return evaluator.Verdict{Feedback: "The output is empty."}, nil
	}
	if len(in.Sources) == 0 {
		return evaluator.Verdict{Feedback: "The output cites no saved source. Use a research tool and base the output on what it returns."}, nil
	}
	lower := strings.ToLower(in.Content)
	for _, claim := range in.Facts.ProhibitedClaims {
		if claim != "" && strings.Contains(lower, strings.ToLower(claim)) {
			return evaluator.Verdict{Feedback: "The output makes a prohibited claim: " + claim}, nil
		}
	}
	return evaluator.Verdict{Pass: true}, nil
}
