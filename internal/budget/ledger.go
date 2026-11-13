package budget

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/RoundtableCI/MerlinCI/internal/config"
)

// Micros is money in millionths of the budget currency. Inference calls cost
// fractions of the smallest coin, so the ledger keeps finer units than the
// configuration does.
type Micros int64

// FromAmount converts a configured amount to micros.
func FromAmount(a config.Amount) Micros { return Micros(a) * 10_000 }

func (m Micros) String() string {
	return fmt.Sprintf("%d.%06d", int64(m)/1_000_000, int64(m)%1_000_000)
}

// Spend categories.
const (
	Inference = "inference"
	Research  = "research"
)

// ErrNoBudget means a category has no limit configured. Spend without a cap
// is refused rather than allowed.
var ErrNoBudget = errors.New("no budget is configured for this category")

// LimitError means a reservation would take spend past a limit.
type LimitError struct {
	Category string
	Period   string
	Limit    Micros
	Used     Micros // settled spend plus open reservations
	Asked    Micros
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("%s budget per %s reached: limit %s, used %s, asked %s",
		e.Category, e.Period, e.Limit, e.Used, e.Asked)
}

// Alert is raised once when spend first crosses a threshold of a limit
// within a period.
type Alert struct {
	Category  string
	Period    string
	Threshold float64
	Limit     Micros
	Spent     Micros
}

// Reservation holds part of a budget for a call that is about to be made.
type Reservation struct {
	ID       int64
	Category string
	Amount   Micros
}

type entry struct {
	category string
	amount   Micros
	at       time.Time
}

// Ledger caps spend before it is spent (deck slide 15). A caller reserves
// the most a call could cost, makes the call, then settles what it did cost.
// The limits come from configuration; nothing in the ledger raises them.
//
// This ledger keeps its state in memory, for tests and local runs. The
// PostgreSQL ledger in package store enforces the same rules across processes.
type Ledger struct {
	mu      sync.Mutex
	spec    config.BudgetSpec
	now     func() time.Time
	onAlert func(Alert)
	entries []entry
	open    map[int64]*Reservation
	nextID  int64
	alerted map[string]bool
}

// NewLedger returns a ledger for the given budget. onAlert may be nil.
func NewLedger(spec config.BudgetSpec, now func() time.Time, onAlert func(Alert)) *Ledger {
	if now == nil {
		now = time.Now
	}
	return &Ledger{spec: spec, now: now, onAlert: onAlert, open: map[int64]*Reservation{}, alerted: map[string]bool{}}
}

// Reserve holds amount against every limit of the category, or fails without
// holding anything.
func (l *Ledger) Reserve(_ context.Context, category string, amount Micros) (*Reservation, error) {
	if amount < 0 {
		return nil, fmt.Errorf("cannot reserve a negative amount")
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	limits := 0
	for _, lim := range l.spec.Limits {
		if lim.Category != category {
			continue
		}
		limits++
		used := l.spent(category, lim.Period, now) + l.reserved(category)
		if limit := FromAmount(lim.Amount); used+amount > limit {
			return nil, &LimitError{Category: category, Period: lim.Period, Limit: limit, Used: used, Asked: amount}
		}
	}
	if limits == 0 {
		return nil, fmt.Errorf("%s: %w", category, ErrNoBudget)
	}
	l.nextID++
	r := &Reservation{ID: l.nextID, Category: category, Amount: amount}
	l.open[r.ID] = r
	return r, nil
}

// Settle records what a reserved call actually cost and frees the reservation.
// The actual cost is always recorded, even when it exceeds the reservation.
func (l *Ledger) Settle(_ context.Context, r *Reservation, actual Micros) error {
	l.mu.Lock()
	if _, ok := l.open[r.ID]; !ok {
		l.mu.Unlock()
		return nil
	}
	delete(l.open, r.ID)
	now := l.now()
	l.entries = append(l.entries, entry{category: r.Category, amount: actual, at: now})
	alerts := l.newAlerts(r.Category, now)
	l.mu.Unlock()

	if l.onAlert != nil {
		for _, a := range alerts {
			l.onAlert(a)
		}
	}
	return nil
}

// Release frees a reservation for a call that was never made.
func (l *Ledger) Release(_ context.Context, r *Reservation) error {
	l.mu.Lock()
	delete(l.open, r.ID)
	l.mu.Unlock()
	return nil
}

// Spent returns settled spend for a category in the current period.
func (l *Ledger) Spent(category, period string) Micros {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.spent(category, period, l.now())
}

func (l *Ledger) spent(category, period string, now time.Time) Micros {
	start := PeriodStart(period, now)
	var sum Micros
	for _, e := range l.entries {
		if e.category == category && !e.at.Before(start) {
			sum += e.amount
		}
	}
	return sum
}

func (l *Ledger) reserved(category string) Micros {
	var sum Micros
	for _, r := range l.open {
		if r.Category == category {
			sum += r.Amount
		}
	}
	return sum
}

func (l *Ledger) newAlerts(category string, now time.Time) []Alert {
	var out []Alert
	for _, lim := range l.spec.Limits {
		if lim.Category != category {
			continue
		}
		limit := FromAmount(lim.Amount)
		spent := l.spent(category, lim.Period, now)
		for _, t := range l.spec.AlertThresholds {
			key := fmt.Sprintf("%s/%s/%v/%d", category, lim.Period, t, PeriodStart(lim.Period, now).Unix())
			if !l.alerted[key] && float64(spent) >= t*float64(limit) {
				l.alerted[key] = true
				out = append(out, Alert{Category: category, Period: lim.Period, Threshold: t, Limit: limit, Spent: spent})
			}
		}
	}
	return out
}

// PeriodStart returns the start of the day or month containing now, in UTC.
func PeriodStart(period string, now time.Time) time.Time {
	now = now.UTC()
	if period == "month" {
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}
