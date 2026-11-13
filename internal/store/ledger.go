package store

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/RoundtableCI/MerlinCI/internal/budget"
	"github.com/RoundtableCI/MerlinCI/internal/config"
)

// Ledger is the budget ledger in PostgreSQL. It enforces the same rules as
// budget.Ledger, and holds across processes and restarts.
type Ledger struct {
	s       *Store
	spec    config.BudgetSpec
	now     func() time.Time
	onAlert func(budget.Alert)
}

// Ledger returns the budget ledger. now and onAlert may be nil.
func (s *Store) Ledger(spec config.BudgetSpec, now func() time.Time, onAlert func(budget.Alert)) *Ledger {
	if now == nil {
		now = time.Now
	}
	return &Ledger{s: s, spec: spec, now: now, onAlert: onAlert}
}

// used returns settled spend since the start of the period ($2), plus the
// holds that count against it: every hold made in the period, and every
// hold recent enough ($3) that its call may still be in flight. Without
// the second part, a call reserved just before midnight would be invisible
// to the new day until it settled.
const usedSQL = `
	SELECT COALESCE((SELECT SUM(amount_micros) FROM budget_entries      WHERE category = $1 AND at >= $2), 0)
	     + COALESCE((SELECT SUM(amount_micros) FROM budget_reservations WHERE category = $1 AND at >= LEAST($2::timestamptz, $3::timestamptz)), 0)`

// inFlight is how long a hold is assumed to belong to a call that may
// still settle. It is longer than any model call is allowed to take.
const inFlight = time.Hour

// Reserve holds amount against every limit of the category, or fails without
// holding anything. Reservations for one category are serialized, so two
// processes cannot both take the last of a budget.
func (l *Ledger) Reserve(ctx context.Context, category string, amount budget.Micros) (*budget.Reservation, error) {
	if amount < 0 {
		return nil, fmt.Errorf("cannot reserve a negative amount")
	}
	now := l.now()
	tx, err := l.s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext('merlin.budget.' || $1))", category); err != nil {
		return nil, err
	}
	limits := 0
	for _, lim := range l.spec.Limits {
		if lim.Category != category {
			continue
		}
		limits++
		var used int64
		if err := tx.QueryRow(ctx, usedSQL, category, budget.PeriodStart(lim.Period, now), now.Add(-inFlight)).Scan(&used); err != nil {
			return nil, err
		}
		if limit := budget.FromAmount(lim.Amount); budget.Micros(used)+amount > limit {
			return nil, &budget.LimitError{Category: category, Period: lim.Period, Limit: limit, Used: budget.Micros(used), Asked: amount}
		}
	}
	if limits == 0 {
		return nil, fmt.Errorf("%s: %w", category, budget.ErrNoBudget)
	}
	r := &budget.Reservation{Category: category, Amount: amount}
	err = tx.QueryRow(ctx,
		"INSERT INTO budget_reservations (category, amount_micros, at) VALUES ($1, $2, $3) RETURNING id",
		category, int64(amount), now).Scan(&r.ID)
	if err != nil {
		return nil, err
	}
	return r, tx.Commit(ctx)
}

// Settle records what a reserved call actually cost and frees the hold, in
// one transaction.
func (l *Ledger) Settle(ctx context.Context, r *budget.Reservation, actual budget.Micros) error {
	now := l.now()
	var alerts []budget.Alert
	err := pgx.BeginFunc(ctx, l.s.pool, func(tx pgx.Tx) error {
		alerts = nil
		tag, err := tx.Exec(ctx, "DELETE FROM budget_reservations WHERE id = $1", r.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil // already settled or released
		}
		if _, err := tx.Exec(ctx,
			"INSERT INTO budget_entries (category, amount_micros, at) VALUES ($1, $2, $3)",
			r.Category, int64(actual), now); err != nil {
			return err
		}
		for _, lim := range l.spec.Limits {
			if lim.Category != r.Category {
				continue
			}
			start := budget.PeriodStart(lim.Period, now)
			var spent int64
			err := tx.QueryRow(ctx,
				"SELECT COALESCE(SUM(amount_micros), 0) FROM budget_entries WHERE category = $1 AND at >= $2",
				r.Category, start).Scan(&spent)
			if err != nil {
				return err
			}
			limit := budget.FromAmount(lim.Amount)
			for _, t := range l.spec.AlertThresholds {
				if float64(spent) < t*float64(limit) {
					continue
				}
				tag, err := tx.Exec(ctx, `
					INSERT INTO budget_alerts (category, period, threshold, period_start)
					VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
					r.Category, lim.Period, strconv.FormatFloat(t, 'g', -1, 64), start)
				if err != nil {
					return err
				}
				if tag.RowsAffected() == 1 {
					alerts = append(alerts, budget.Alert{Category: r.Category, Period: lim.Period, Threshold: t, Limit: limit, Spent: budget.Micros(spent)})
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if l.onAlert != nil {
		for _, a := range alerts {
			l.onAlert(a)
		}
	}
	return nil
}

// Release frees a hold for a call that was never made.
func (l *Ledger) Release(ctx context.Context, r *budget.Reservation) error {
	_, err := l.s.pool.Exec(ctx, "DELETE FROM budget_reservations WHERE id = $1", r.ID)
	return err
}

// Spent returns settled spend for a category in the current period.
func (l *Ledger) Spent(ctx context.Context, category, period string) (budget.Micros, error) {
	var spent int64
	err := l.s.pool.QueryRow(ctx,
		"SELECT COALESCE(SUM(amount_micros), 0) FROM budget_entries WHERE category = $1 AND at >= $2",
		category, budget.PeriodStart(period, l.now())).Scan(&spent)
	return budget.Micros(spent), err
}
