// Package researchtools holds the research tools: web search and page fetch.
// Both are in the automatic tier of the default policy (deck slide 11:
// research and competitor monitoring run without asking).
package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/RoundtableCI/MerlinCI/internal/integrations/research"
	"github.com/RoundtableCI/MerlinCI/pkg/tool"
)

// Charge reserves and records the cost of one paid call, or returns an
// error when the research budget cannot cover it.
type Charge func(ctx context.Context) error

// Search is the web-search tool.
type Search struct {
	Searcher research.Searcher
	// Charge is called before every search. The search does not happen
	// when it fails.
	Charge Charge
}

// Spec implements tool.Tool.
func (Search) Spec() tool.Spec {
	return tool.Spec{
		Name:        "web-search",
		Description: "Search the public web. Returns titles, URLs and short snippets. Snippets are not evidence: fetch a page before relying on it.",
		Action:      "research.search",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"additionalProperties": false,
			"required": ["query"],
			"properties": {
				"query": {"type": "string", "minLength": 2, "maxLength": 200},
				"limit": {"type": "integer", "minimum": 1, "maximum": 10}
			}
		}`),
	}
}

// Call implements tool.Tool.
func (s Search) Call(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, err
	}
	if in.Limit == 0 {
		in.Limit = 5
	}
	if err := s.Charge(ctx); err != nil {
		return tool.Result{}, fmt.Errorf("research budget: %w", err)
	}
	results, err := s.Searcher.Search(ctx, in.Query, in.Limit)
	if err != nil {
		return tool.Result{}, err
	}
	if len(results) == 0 {
		return tool.Result{Content: "No results."}, nil
	}
	var b strings.Builder
	for i, r := range results {
		fmt.Fprintf(&b, "%d. %s\n   %s\n   %s\n", i+1, r.Title, r.URL, r.Snippet)
	}
	return tool.Result{Content: b.String()}, nil
}

// Fetch is the fetch-page tool. A fetched page is evidence: it is saved as
// a source, with the hash of what was read.
type Fetch struct {
	Fetcher *research.Fetcher
}

// Spec implements tool.Tool.
func (Fetch) Spec() tool.Spec {
	return tool.Spec{
		Name:        "fetch-page",
		Description: "Read one public web page as text. The page is saved as a source for your output.",
		Action:      "research.fetch",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"additionalProperties": false,
			"required": ["url"],
			"properties": {
				"url": {"type": "string", "pattern": "^https?://", "maxLength": 2000}
			}
		}`),
	}
}

const excerptRunes = 500

// Call implements tool.Tool.
func (f Fetch) Call(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, err
	}
	page, err := f.Fetcher.Fetch(ctx, in.URL)
	if err != nil {
		return tool.Result{}, err
	}
	excerpt := page.Text
	if r := []rune(excerpt); len(r) > excerptRunes {
		excerpt = string(r[:excerptRunes])
	}
	return tool.Result{
		Content: "Title: " + page.Title + "\nURL: " + page.URL + "\n\n" + page.Text,
		Sources: []tool.Source{{
			URL:         page.URL,
			Title:       page.Title,
			RetrievedAt: page.RetrievedAt,
			SHA256:      page.SHA256,
			Excerpt:     excerpt,
		}},
	}, nil
}
