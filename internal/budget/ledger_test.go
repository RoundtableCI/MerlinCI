package budget

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RoundtableCI/MerlinCI/internal/config"
)

// The deck's defaults: 150 per day and 3,000 per month for inference, 500
// per month for research, alerts at 70% and 90%.
func deckBudget() config.BudgetSpec {
	return config.BudgetSpec{
		Currency: "INR",
		Limits: []config.Limit{
			{Category: Inference, Period: "day", Amount: 15000},
			{Category: Inference, Period: "month", Amount: 300000},
			{Category: Research, Period: "month", Amount: 50000},
		},
		AlertThresholds: []float64{0.7, 0.9},
		AtLimit:         "stop",
	}
}

const rupee = Micros(1_000_000)

var ctx = context.Background()

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func TestReserveRefusesPastTheLimit(t *testing.T) {
	c := &clock{time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)}
	l := NewLedger(deckBudget(), c.now, nil)

	r, err := l.Reserve(ctx, Inference, 100*rupee)
	if err != nil {
		t.Fatal(err)
	}
	// 100 held plus 51 asked is past the 150 daily limit, though nothing is settled yet.
	_, err = l.Reserve(ctx, Inference, 51*rupee)
	var limit *LimitError
	if !errors.As(err, &limit) || limit.Period != "day" || limit.Limit != 150*rupee {
		t.Fatalf("err = %v, want a daily limit error", err)
	}
	if _, err := l.Reserve(ctx, Inference, 50*rupee); err != nil {
		t.Fatalf("a reservation that exactly reaches the limit was refused: %v", err)
	}
	_ = l.Settle(ctx, r, 40*rupee)
	if got := l.Spent(Inference, "day"); got != 40*rupee {
		t.Errorf("spent = %s, want 40", got)
	}
}

func TestReleaseFreesTheHold(t *testing.T) {
	l := NewLedger(deckBudget(), nil, nil)
	r, _ := l.Reserve(ctx, Inference, 150*rupee)
	if _, err := l.Reserve(ctx, Inference, rupee); err == nil {
		t.Fatal("reserved past a fully held limit")
	}
	_ = l.Release(ctx, r)
	if _, err := l.Reserve(ctx, Inference, 150*rupee); err != nil {
		t.Fatalf("limit still held after release: %v", err)
	}
	if got := l.Spent(Inference, "day"); got != 0 {
		t.Errorf("a released reservation was charged: %s", got)
	}
}

func TestDayResetsButMonthDoesNot(t *testing.T) {
	c := &clock{time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)}
	l := NewLedger(deckBudget(), c.now, nil)

	// Spend the full daily limit on 20 days: 3,000, the monthly ceiling.
	for day := 0; day < 20; day++ {
		r, err := l.Reserve(ctx, Inference, 150*rupee)
		if err != nil {
			t.Fatalf("day %d: %v", day+1, err)
		}
		_ = l.Settle(ctx, r, 150*rupee)
		c.t = c.t.AddDate(0, 0, 1)
	}
	_, err := l.Reserve(ctx, Inference, rupee)
	var limit *LimitError
	if !errors.As(err, &limit) || limit.Period != "month" {
		t.Fatalf("err = %v, want a monthly limit error", err)
	}
	c.t = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	if _, err := l.Reserve(ctx, Inference, rupee); err != nil {
		t.Fatalf("a new month did not reset the monthly limit: %v", err)
	}
}

func TestCategoriesAreSeparateAndUncappedSpendIsRefused(t *testing.T) {
	l := NewLedger(deckBudget(), nil, nil)
	r, _ := l.Reserve(ctx, Inference, 150*rupee)
	_ = l.Settle(ctx, r, 150*rupee)
	if _, err := l.Reserve(ctx, Research, 500*rupee); err != nil {
		t.Fatalf("inference spend counted against research: %v", err)
	}
	if _, err := l.Reserve(ctx, "ads", rupee); !errors.Is(err, ErrNoBudget) {
		t.Fatalf("err = %v, want ErrNoBudget for a category with no limit", err)
	}
}

func TestAlertsFireOncePerThreshold(t *testing.T) {
	var alerts []Alert
	l := NewLedger(deckBudget(), nil, func(a Alert) { alerts = append(alerts, a) })
	spend := func(amount Micros) {
		r, err := l.Reserve(ctx, Research, amount)
		if err != nil {
			t.Fatal(err)
		}
		_ = l.Settle(ctx, r, amount)
	}
	spend(349 * rupee) // 69.8%
	if len(alerts) != 0 {
		t.Fatalf("alert below 70%%: %+v", alerts)
	}
	spend(1 * rupee) // 70%
	spend(1 * rupee) // still past 70%, no second alert
	if len(alerts) != 1 || alerts[0].Threshold != 0.7 || alerts[0].Category != Research {
		t.Fatalf("alerts = %+v, want one at 70%%", alerts)
	}
	spend(100 * rupee) // 90.2%
	if len(alerts) != 2 || alerts[1].Threshold != 0.9 {
		t.Fatalf("alerts = %+v, want a second at 90%%", alerts)
	}
}

func TestMicros(t *testing.T) {
	if got := FromAmount(15005); got != 150_050_000 {
		t.Errorf("FromAmount(150.05) = %d", got)
	}
	if got := Micros(252000).String(); got != "0.252000" {
		t.Errorf("String() = %q", got)
	}
}
