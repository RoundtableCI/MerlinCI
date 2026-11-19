package gateway

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/RoundtableCI/MerlinCI/internal/budget"
	"github.com/RoundtableCI/MerlinCI/internal/config"
	"github.com/RoundtableCI/MerlinCI/pkg/provider"
)

type stubProvider struct {
	usage provider.Usage
	err   error
	calls int
	got   provider.Request
}

func (s *stubProvider) Name() string { return "stub" }

func (s *stubProvider) Complete(_ context.Context, req provider.Request) (provider.Response, error) {
	s.calls++
	s.got = req
	return provider.Response{Message: provider.Message{Content: "ok"}, Usage: s.usage}, s.err
}

func spec(dailyLimit config.Amount) config.BudgetSpec {
	return config.BudgetSpec{
		Currency:      "INR",
		Limits:        []config.Limit{{Category: "inference", Period: "day", Amount: dailyLimit}},
		ExchangeRates: map[string]config.Rate{"USD": 900000},
	}
}

func newGateway(t *testing.T, p provider.Provider, dailyLimit config.Amount) (*Gateway, *budget.Ledger) {
	t.Helper()
	b := spec(dailyLimit)
	ledger := budget.NewLedger(b, nil, nil)
	rate, err := RateFor(b, "USD")
	if err != nil {
		t.Fatal(err)
	}
	g, err := New(Config{
		Provider: p,
		Models:   []Model{{ID: "small", InputPerMTok: 1_000_000, OutputPerMTok: 2_000_000}, {ID: "large", InputPerMTok: 10_000_000, OutputPerMTok: 30_000_000}},
		Currency: "USD",
		Rate:     rate,
		Ledger:   ledger,
	})
	if err != nil {
		t.Fatal(err)
	}
	return g, ledger
}

var hello = provider.Request{Messages: []provider.Message{{Role: provider.RoleUser, Content: "hello"}}}

func TestCostFromTokensWhenProviderReportsNone(t *testing.T) {
	p := &stubProvider{usage: provider.Usage{InputTokens: 1000, OutputTokens: 200}}
	g, ledger := newGateway(t, p, 15000)
	res, err := g.Complete(context.Background(), Request{Request: hello})
	if err != nil {
		t.Fatal(err)
	}
	// $0.0014 at 90 INR to the dollar is 0.126 INR.
	if res.Cost != 126_000 || ledger.Spent(budget.Inference, "day") != 126_000 {
		t.Errorf("cost %d, ledger %d; want 126000 for both", res.Cost, ledger.Spent(budget.Inference, "day"))
	}
	if p.got.Model != "small" || p.got.MaxOutputTokens != DefaultMaxOutputTokens {
		t.Errorf("request went out as model %q with max tokens %d; want the default model and a bound", p.got.Model, p.got.MaxOutputTokens)
	}
}

func TestProviderReportedCostIsUsed(t *testing.T) {
	p := &stubProvider{usage: provider.Usage{InputTokens: 1000, OutputTokens: 200, CostMicros: 5000, Currency: "USD"}}
	g, _ := newGateway(t, p, 15000)
	res, err := g.Complete(context.Background(), Request{Request: hello})
	if err != nil {
		t.Fatal(err)
	}
	if res.Cost != 450_000 {
		t.Errorf("cost = %d, want 450000 ($0.005 at 90)", res.Cost)
	}
}

func TestModelOffTheAllowlistIsRefused(t *testing.T) {
	p := &stubProvider{}
	g, _ := newGateway(t, p, 15000)
	req := hello
	req.Model = "someone-elses-model"
	if _, err := g.Complete(context.Background(), Request{Request: req}); !errors.Is(err, ErrModelNotAllowed) {
		t.Fatalf("err = %v, want ErrModelNotAllowed", err)
	}
	if p.calls != 0 {
		t.Error("the provider was called with a model off the allowlist")
	}
}

func TestBudgetIsCheckedBeforeTheCall(t *testing.T) {
	p := &stubProvider{}
	g, ledger := newGateway(t, p, 1) // 0.01 INR per day
	_, err := g.Complete(context.Background(), Request{Request: hello})
	var limit *budget.LimitError
	if !errors.As(err, &limit) {
		t.Fatalf("err = %v, want a limit error", err)
	}
	if p.calls != 0 || ledger.Spent(budget.Inference, "day") != 0 {
		t.Error("the provider was called although the budget could not cover the worst case")
	}
}

func TestTaskCapIsCheckedBeforeTheCall(t *testing.T) {
	p := &stubProvider{}
	g, _ := newGateway(t, p, 15000)
	if _, err := g.Complete(context.Background(), Request{Request: hello, CostCap: 10}); !errors.Is(err, ErrTaskBudget) {
		t.Fatalf("err = %v, want ErrTaskBudget", err)
	}
	if p.calls != 0 {
		t.Error("the provider was called past the task cap")
	}
}

// A call the provider refused cost nothing, and is not charged.
func TestFailedCallIsNotCharged(t *testing.T) {
	p := &stubProvider{err: fmt.Errorf("%w: status 503", provider.ErrNotBilled)}
	g, ledger := newGateway(t, p, 15000)
	if _, err := g.Complete(context.Background(), Request{Request: hello}); err == nil {
		t.Fatal("no error from a failed call")
	}
	if got := ledger.Spent(budget.Inference, "day"); got != 0 {
		t.Errorf("a failed call was charged %d", got)
	}
	// The hold was released: the whole day's budget is available again.
	if _, err := ledger.Reserve(context.Background(), budget.Inference, 150_000_000); err != nil {
		t.Errorf("the reservation of a failed call was not released: %v", err)
	}
}

func TestRateFor(t *testing.T) {
	b := spec(15000)
	if r, err := RateFor(b, "INR"); err != nil || r != config.RateOne {
		t.Errorf("same currency: %v, %v", r, err)
	}
	if _, err := RateFor(b, "EUR"); err == nil {
		t.Error("a currency with no configured rate was accepted")
	}
}

func TestGatewayNeedsAnAllowlist(t *testing.T) {
	ledger := budget.NewLedger(spec(15000), nil, nil)
	if _, err := New(Config{Provider: &stubProvider{}, Ledger: ledger, Rate: config.RateOne}); err == nil {
		t.Error("a gateway with no allowed models was created")
	}
}

// A timeout or a dropped connection can follow a request the provider has
// already billed. Nobody can tell, so the worst case is counted as spent.
func TestUncertainFailureIsChargedAsWorstCase(t *testing.T) {
	p := &stubProvider{err: errors.New("context deadline exceeded while reading the response")}
	g, ledger := newGateway(t, p, 15000)
	_, err := g.Complete(context.Background(), Request{Request: hello})
	var charged *ChargedError
	if !errors.As(err, &charged) || charged.Cost <= 0 {
		t.Fatalf("err = %v, want a ChargedError with a cost", err)
	}
	if got := ledger.Spent(budget.Inference, "day"); got != charged.Cost {
		t.Errorf("ledger spent %d, error says %d was charged", got, charged.Cost)
	}
	// Nothing is left on hold: the worst case was settled, not kept reserved.
	if _, err := ledger.Reserve(context.Background(), budget.Inference, 150_000_000-charged.Cost); err != nil {
		t.Errorf("the rest of the day's budget is not available: %v", err)
	}
}

func TestMissingOrZeroCostIsNeverFree(t *testing.T) {
	// Tokens but a reported cost of zero: priced from the tokens.
	p := &stubProvider{usage: provider.Usage{InputTokens: 1000, OutputTokens: 200, CostMicros: 0, Currency: "USD"}}
	g, _ := newGateway(t, p, 15000)
	res, err := g.Complete(context.Background(), Request{Request: hello})
	if err != nil || res.Cost != 126_000 {
		t.Errorf("zero reported cost: charged %d, %v; want 126000 from the token count", res.Cost, err)
	}

	// Neither a cost nor a token count: charged as the worst case.
	p = &stubProvider{}
	g, ledger := newGateway(t, p, 15000)
	res, err = g.Complete(context.Background(), Request{Request: hello})
	if err != nil || res.Cost <= 0 || ledger.Spent(budget.Inference, "day") != res.Cost {
		t.Errorf("no usage reported: charged %d, %v; want the worst case", res.Cost, err)
	}
}

// The reservation must be an upper bound whatever the text is made of.
func TestInputEstimateIsAnUpperBound(t *testing.T) {
	// One byte is the shortest a token can be.
	dense := provider.Request{Messages: []provider.Message{{Role: provider.RoleUser, Content: "{[(<>)]}{[(<>)]}"}}}
	if got := estimateInputTokens(dense); got < len("{[(<>)]}{[(<>)]}") {
		t.Errorf("estimate %d is below the %d bytes of the content", got, len("{[(<>)]}{[(<>)]}"))
	}
}
