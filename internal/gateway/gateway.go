package gateway

import (
	"context"
	"errors"
	"fmt"

	"github.com/RoundtableCI/MerlinCI/internal/budget"
	"github.com/RoundtableCI/MerlinCI/internal/config"
	"github.com/RoundtableCI/MerlinCI/pkg/provider"
)

// Model is one entry of the model allowlist, with its price in millionths of
// the provider's currency per million tokens.
type Model struct {
	ID            string
	InputPerMTok  int64
	OutputPerMTok int64
}

// Ledger is the part of the budget ledger the gateway needs.
type Ledger interface {
	Reserve(ctx context.Context, category string, amount budget.Micros) (*budget.Reservation, error)
	Settle(ctx context.Context, r *budget.Reservation, actual budget.Micros) error
	Release(ctx context.Context, r *budget.Reservation) error
}

var (
	// ErrModelNotAllowed means the requested model is not on the allowlist.
	ErrModelNotAllowed = errors.New("model is not on the allowlist")
	// ErrTaskBudget means the call could cost more than the task has left.
	ErrTaskBudget = errors.New("the call could exceed the task budget")
)

// ChargedError is a failed call that was counted as spent, because the
// provider may have billed it. Cost is what was counted.
type ChargedError struct {
	Cost budget.Micros
	Err  error
}

func (e *ChargedError) Error() string { return e.Err.Error() }

func (e *ChargedError) Unwrap() error { return e.Err }

// DefaultMaxOutputTokens bounds a request that sets no bound of its own. An
// unbounded request has no worst-case cost, so none is ever sent.
const DefaultMaxOutputTokens = 1024

// Config configures a Gateway.
type Config struct {
	Provider provider.Provider
	// Models is the allowlist. The first model is the default.
	Models []Model
	// Currency is the currency the provider bills in.
	Currency string
	// Rate converts the provider's currency to the budget currency.
	Rate   config.Rate
	Ledger Ledger
}

// Gateway is the only path to a model. It enforces the allowlist and
// reserves the worst-case cost of every call before the call is made.
type Gateway struct {
	cfg    Config
	models map[string]Model
}

// New returns a gateway, or an error when the configuration cannot cap spend.
func New(cfg Config) (*Gateway, error) {
	if cfg.Provider == nil || cfg.Ledger == nil {
		return nil, errors.New("gateway needs a provider and a ledger")
	}
	if len(cfg.Models) == 0 {
		return nil, errors.New("gateway needs at least one allowed model")
	}
	if cfg.Rate <= 0 {
		return nil, errors.New("gateway needs a positive exchange rate")
	}
	g := &Gateway{cfg: cfg, models: map[string]Model{}}
	for _, m := range cfg.Models {
		if m.InputPerMTok < 0 || m.OutputPerMTok < 0 {
			return nil, fmt.Errorf("model %q has a negative price", m.ID)
		}
		g.models[m.ID] = m
	}
	return g, nil
}

// RateFor returns the rate from a provider currency to the budget currency.
func RateFor(spec config.BudgetSpec, providerCurrency string) (config.Rate, error) {
	if providerCurrency == spec.Currency {
		return config.RateOne, nil
	}
	if r, ok := spec.ExchangeRates[providerCurrency]; ok && r > 0 {
		return r, nil
	}
	return 0, fmt.Errorf("the budget has no exchange rate for %s", providerCurrency)
}

// Request is an inference request with the most it may cost.
type Request struct {
	provider.Request
	// CostCap is what the task has left to spend. Zero means the task sets
	// no cap; the budget ledger still applies.
	CostCap budget.Micros
}

// Result is a response and what it cost in the budget currency.
type Result struct {
	provider.Response
	Model string
	Cost  budget.Micros
}

// Complete runs one inference call inside the budget.
func (g *Gateway) Complete(ctx context.Context, req Request) (Result, error) {
	if req.Model == "" {
		req.Model = g.cfg.Models[0].ID
	}
	model, ok := g.models[req.Model]
	if !ok {
		return Result{}, fmt.Errorf("%q: %w", req.Model, ErrModelNotAllowed)
	}
	if req.MaxOutputTokens <= 0 {
		req.MaxOutputTokens = DefaultMaxOutputTokens
	}

	worst := g.convert(price(model, estimateInputTokens(req.Request), req.MaxOutputTokens))
	if req.CostCap > 0 && worst > req.CostCap {
		return Result{}, fmt.Errorf("worst case %s, task has %s left: %w", worst, req.CostCap, ErrTaskBudget)
	}
	reservation, err := g.cfg.Ledger.Reserve(ctx, budget.Inference, worst)
	if err != nil {
		return Result{}, err
	}

	resp, err := g.cfg.Provider.Complete(ctx, req.Request)
	if err != nil {
		// These use their own context: the call's context may be the
		// reason the call failed. If they fail too, the hold stays, which
		// over-counts spend rather than under-counting it.
		settled := context.WithoutCancel(ctx)
		if errors.Is(err, provider.ErrNotBilled) || errors.Is(err, provider.ErrCredential) {
			_ = g.cfg.Ledger.Release(settled, reservation)
			return Result{}, fmt.Errorf("%s: %w", g.cfg.Provider.Name(), err)
		}
		// A timeout, a dropped connection or an unreadable reply can all
		// follow a request the provider has already charged for. Nobody
		// can tell, so the worst case is counted as spent.
		_ = g.cfg.Ledger.Settle(settled, reservation, worst)
		return Result{}, &ChargedError{Cost: worst, Err: fmt.Errorf("%s: %w", g.cfg.Provider.Name(), err)}
	}

	var actual budget.Micros
	switch {
	case resp.Usage.Currency == g.cfg.Currency && resp.Usage.CostMicros > 0:
		actual = g.convert(resp.Usage.CostMicros)
	case resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0:
		// The provider reported no cost, so compute it from the
		// allowlist price.
		actual = g.convert(price(model, resp.Usage.InputTokens, resp.Usage.OutputTokens))
	default:
		// A reply with neither a cost nor a token count was not free.
		actual = worst
	}
	// The money is spent, so the settlement must not be lost to a
	// cancelled context. If it still fails, the hold stays in place.
	if err := g.cfg.Ledger.Settle(context.WithoutCancel(ctx), reservation, actual); err != nil {
		return Result{}, fmt.Errorf("record spend: %w", err)
	}
	return Result{Response: resp, Model: req.Model, Cost: actual}, nil
}

// price returns the cost of a call in provider micros, rounded up.
func price(m Model, inputTokens, outputTokens int) int64 {
	total := int64(inputTokens)*m.InputPerMTok + int64(outputTokens)*m.OutputPerMTok
	return (total + 999_999) / 1_000_000
}

// convert turns provider micros into budget micros, rounded up.
func (g *Gateway) convert(providerMicros int64) budget.Micros {
	r := int64(g.cfg.Rate)
	return budget.Micros((providerMicros*r + int64(config.RateOne) - 1) / int64(config.RateOne))
}

// estimateInputTokens over-estimates the input size of a request. A token
// is never shorter than one byte, so the byte count is an upper bound for
// any text and any tokenizer, which keeps the reservation a worst case.
func estimateInputTokens(req provider.Request) int {
	bytes := 0
	for _, m := range req.Messages {
		bytes += len(m.Content) + 16
		for _, c := range m.ToolCalls {
			bytes += len(c.Name) + len(c.Arguments) + 16
		}
	}
	for _, t := range req.Tools {
		bytes += len(t.Name) + len(t.Description) + len(t.InputSchema) + 16
	}
	return bytes
}

// FromConfig builds a gateway from the model allowlist and the budget. The
// default model comes first, so a request that names no model uses it.
func FromConfig(models config.ModelsSpec, budgetSpec config.BudgetSpec, p provider.Provider, ledger Ledger) (*Gateway, error) {
	rate, err := RateFor(budgetSpec, models.Currency)
	if err != nil {
		return nil, err
	}
	var list []Model
	for _, m := range models.Models {
		entry := Model{ID: m.ID, InputPerMTok: int64(m.InputPerMillionTokens), OutputPerMTok: int64(m.OutputPerMillionTokens)}
		if m.ID == models.Default {
			list = append([]Model{entry}, list...)
		} else {
			list = append(list, entry)
		}
	}
	return New(Config{Provider: p, Models: list, Currency: models.Currency, Rate: rate, Ledger: ledger})
}
