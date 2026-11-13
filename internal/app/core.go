package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/RoundtableCI/MerlinCI/internal/artifacts"
	"github.com/RoundtableCI/MerlinCI/internal/blob"
	"github.com/RoundtableCI/MerlinCI/internal/blob/s3blob"
	"github.com/RoundtableCI/MerlinCI/internal/budget"
	"github.com/RoundtableCI/MerlinCI/internal/config"
	"github.com/RoundtableCI/MerlinCI/internal/eval"
	"github.com/RoundtableCI/MerlinCI/internal/eval/brandguardian"
	"github.com/RoundtableCI/MerlinCI/internal/gateway"
	"github.com/RoundtableCI/MerlinCI/internal/gateway/openrouter"
	"github.com/RoundtableCI/MerlinCI/internal/integrations/credentials"
	"github.com/RoundtableCI/MerlinCI/internal/integrations/research"
	"github.com/RoundtableCI/MerlinCI/internal/policy"
	"github.com/RoundtableCI/MerlinCI/internal/runtime"
	"github.com/RoundtableCI/MerlinCI/internal/store"
	"github.com/RoundtableCI/MerlinCI/internal/tools"
	"github.com/RoundtableCI/MerlinCI/internal/tools/researchtools"
	"github.com/RoundtableCI/MerlinCI/pkg/evaluator"
	"github.com/RoundtableCI/MerlinCI/pkg/provider"
)

// Options replace the real outside services. Tests use them; the programs
// in cmd/ leave them empty.
type Options struct {
	Provider    provider.Provider
	Searcher    research.Searcher
	Fetcher     *research.Fetcher
	Blobs       blob.Store
	Credentials *credentials.Store
	Log         *slog.Logger
}

// Core is a fully wired MerlinCI: configuration, storage and the runtime.
type Core struct {
	Env     Env
	Config  *config.Bundle
	Store   *store.Store
	Ledger  *store.Ledger
	Blobs   blob.Store
	Runtime *runtime.Runtime
	Log     *slog.Logger
}

// Open loads and checks the configuration, connects to storage, applies
// database migrations and builds the runtime. It refuses to start on a
// configuration with errors, or one that is not a complete deployment.
func Open(ctx context.Context, env Env, opts Options) (*Core, error) {
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}

	cfg, problems := config.Load(env.ConfigDir)
	for _, p := range problems {
		if p.Severity == config.Warning {
			log.Warn("configuration", "problem", p.String())
		}
	}
	if problems.HasErrors() {
		var lines []string
		for _, p := range problems {
			if p.Severity == config.Error {
				lines = append(lines, p.String())
			}
		}
		return nil, fmt.Errorf("configuration in %s is invalid:\n  %s", env.ConfigDir, strings.Join(lines, "\n  "))
	}
	if cfg.Profile == nil || cfg.Policy == nil || cfg.Budget == nil || cfg.Models == nil {
		return nil, fmt.Errorf("configuration in %s needs a Profile, a Policy, a Budget and a Models document", env.ConfigDir)
	}

	st, err := store.Open(ctx, env.DatabaseURL)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Core, error) {
		st.Close()
		return nil, err
	}
	if err := st.Migrate(ctx); err != nil {
		return fail(fmt.Errorf("database migration: %w", err))
	}

	events := st.Events()
	ledger := st.Ledger(cfg.Budget.Spec, nil, func(a budget.Alert) {
		log.Warn("budget alert", "category", a.Category, "period", a.Period, "threshold", a.Threshold,
			"spent", a.Spent.String(), "limit", a.Limit.String())
		_ = events.Record(context.WithoutCancel(ctx), runtime.Event{
			TaskID: "-", Role: "-", Type: "budget alert",
			Data: map[string]any{"category": a.Category, "period": a.Period, "threshold": a.Threshold,
				"spent": a.Spent.String(), "limit": a.Limit.String()},
		})
	})

	blobs := opts.Blobs
	if blobs == nil {
		if blobs, err = s3blob.New(ctx, env.S3); err != nil {
			return fail(err)
		}
	}
	creds := opts.Credentials
	if creds == nil {
		creds = credentials.FromEnv()
	}

	p := opts.Provider
	if p == nil {
		if cfg.Models.Spec.Provider != "openrouter" {
			return fail(fmt.Errorf("model provider %q is not supported; use openrouter", cfg.Models.Spec.Provider))
		}
		if !creds.Has(credentials.OpenRouter) {
			log.Warn("no model credential: runs will fail until it is set", "name", credentials.OpenRouter)
		}
		p = openrouter.New("", creds.Getter(credentials.OpenRouter), nil)
	}
	gw, err := gateway.FromConfig(cfg.Models.Spec, cfg.Budget.Spec, p, ledger)
	if err != nil {
		return fail(err)
	}

	searcher := opts.Searcher
	if searcher == nil {
		if !creds.Has(credentials.BraveSearch) {
			log.Warn("no search credential: web search will fail until it is set", "name", credentials.BraveSearch)
		}
		searcher = research.NewBrave("", creds.Getter(credentials.BraveSearch), nil)
	}
	fetcher := opts.Fetcher
	if fetcher == nil {
		fetcher = research.NewFetcher()
	}
	registry := tools.NewRegistry()
	search := researchtools.Search{Searcher: searcher, Charge: charge(ledger, cfg.Budget.Spec, "research.search")}
	if err := errors.Join(registry.Register(search), registry.Register(researchtools.Fetch{Fetcher: fetcher})); err != nil {
		return fail(err)
	}

	rt := &runtime.Runtime{
		Config:  cfg,
		Gateway: gw,
		Tools:   registry,
		Policy:  policy.New(cfg.Policy.Spec, st.Counter(nil)),
		Evaluators: map[string]evaluator.Evaluator{
			"evidence":       eval.Evidence{},
			"brand-guardian": brandguardian.Guardian{Gateway: gw, Model: cfg.Models.Spec.EvaluatorModel},
		},
		Artifacts:   &artifacts.Store{Blobs: blobs, Index: st.Artifacts()},
		Checkpoints: st.Checkpoints(),
		Effects:     st.Effects(),
		Events:      events,
		Paused:      st.Controls().Paused,
		Log:         log,
	}
	// Every enabled role must be runnable with what is registered here.
	for name, role := range cfg.Roles {
		if !role.Spec.Enabled {
			continue
		}
		for _, t := range role.Spec.Tools {
			if _, ok := registry.Get(t); !ok {
				return fail(fmt.Errorf("role %q is enabled but its tool %q does not exist", name, t))
			}
		}
		for _, e := range role.Spec.Evaluation.Evaluators {
			if _, ok := rt.Evaluators[e]; !ok {
				return fail(fmt.Errorf("role %q is enabled but its evaluator %q does not exist", name, e))
			}
		}
	}
	return &Core{Env: env, Config: cfg, Store: st, Ledger: ledger, Blobs: blobs, Runtime: rt, Log: log}, nil
}

// Close releases the database connections.
func (c *Core) Close() { c.Store.Close() }

// charge returns a function that pays for one call of a paid action out of
// the research budget. An action with no configured cost cannot be paid
// for, so it cannot run.
func charge(ledger *store.Ledger, spec config.BudgetSpec, action string) researchtools.Charge {
	return func(ctx context.Context) error {
		cost, ok := spec.UnitCosts[action]
		if !ok {
			return fmt.Errorf("the budget sets no unit cost for %s", action)
		}
		amount := budget.FromAmount(cost)
		r, err := ledger.Reserve(ctx, budget.Research, amount)
		if err != nil {
			return err
		}
		return ledger.Settle(context.WithoutCancel(ctx), r, amount)
	}
}
