package researchtools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/RoundtableCI/MerlinCI/internal/integrations/research"
	"github.com/RoundtableCI/MerlinCI/internal/tools"
)

var ctx = context.Background()

type fakeSearcher struct {
	calls   int
	results []research.Result
}

func (f *fakeSearcher) Name() string { return "fake" }

func (f *fakeSearcher) Search(_ context.Context, _ string, limit int) ([]research.Result, error) {
	f.calls++
	if limit < len(f.results) {
		return f.results[:limit], nil
	}
	return f.results, nil
}

func TestSearchChargesBeforeItSearches(t *testing.T) {
	s := &fakeSearcher{results: []research.Result{{Title: "A thread", URL: "https://example.com/a", Snippet: "People ask."}}}
	charged := 0
	tl := Search{Searcher: s, Charge: func(context.Context) error { charged++; return nil }}

	out, err := tl.Call(ctx, json.RawMessage(`{"query":"compare models"}`))
	if err != nil {
		t.Fatal(err)
	}
	if charged != 1 || s.calls != 1 {
		t.Errorf("charged %d times for %d searches, want 1 and 1", charged, s.calls)
	}
	if !strings.Contains(out.Content, "https://example.com/a") || !strings.Contains(out.Content, "A thread") {
		t.Errorf("content = %q", out.Content)
	}
	// A snippet is not evidence: only a fetched page is saved as a source.
	if len(out.Sources) != 0 {
		t.Errorf("search saved %d sources, want none", len(out.Sources))
	}
}

func TestSearchDoesNotRunWhenTheBudgetRefuses(t *testing.T) {
	s := &fakeSearcher{}
	tl := Search{Searcher: s, Charge: func(context.Context) error { return errors.New("research budget per month reached") }}
	if _, err := tl.Call(ctx, json.RawMessage(`{"query":"compare models"}`)); err == nil {
		t.Fatal("the search ran although the budget refused")
	}
	if s.calls != 0 {
		t.Errorf("the provider was called %d times, want 0", s.calls)
	}
}

func TestInputSchemas(t *testing.T) {
	r := tools.NewRegistry()
	if err := r.Register(Search{}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(Fetch{}); err != nil {
		t.Fatal(err)
	}
	good := map[string]string{
		"web-search": `{"query":"compare models","limit":10}`,
		"fetch-page": `{"url":"https://example.com/a"}`,
	}
	for name, input := range good {
		if err := r.ValidateInput(name, json.RawMessage(input)); err != nil {
			t.Errorf("%s: valid input rejected: %v", name, err)
		}
	}
	bad := [][2]string{
		{"web-search", `{"query":"x"}`},                                   // too short
		{"web-search", `{"query":"compare models","limit":500}`},          // too many
		{"web-search", `{"query":"compare models","site":"internal"}`},    // unknown field
		{"web-search", `{"query":"` + strings.Repeat("a", 201) + `"}`},    // too long
		{"fetch-page", `{"url":"file:///etc/passwd"}`},                    // not http
		{"fetch-page", `{"url":"https://example.com","method":"DELETE"}`}, // unknown field
		{"fetch-page", `{}`},
	}
	for _, tc := range bad {
		if err := r.ValidateInput(tc[0], json.RawMessage(tc[1])); err == nil {
			t.Errorf("%s accepted %s", tc[0], tc[1])
		}
	}
}
