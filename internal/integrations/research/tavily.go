package research

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// TavilyBaseURL is the Tavily Search API.
const TavilyBaseURL = "https://api.tavily.com"

// Tavily searches the web through the Tavily Search API. It is the search
// placeholder for open decision 07 because its free plan needs no card.
type Tavily struct {
	baseURL string
	key     func() (string, error)
	http    *http.Client
}

// NewTavily returns a Tavily Search client. key supplies the API key from the
// integration layer each time a request is made.
func NewTavily(baseURL string, key func() (string, error), httpClient *http.Client) *Tavily {
	if baseURL == "" {
		baseURL = TavilyBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Tavily{baseURL: baseURL, key: key, http: httpClient}
}

// Name implements Searcher.
func (t *Tavily) Name() string { return "tavily" }

// Search implements Searcher. It uses a basic search, which costs one credit.
func (t *Tavily) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	key, err := t.key()
	if err != nil {
		return nil, errors.New("no search credential is available")
	}
	body, err := json.Marshal(map[string]any{
		"query":        query,
		"max_results":  limit,
		"search_depth": "basic",
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := t.http.Do(req)
	if err != nil {
		// A transport error can quote the request; never let it quote the key.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			return nil, fmt.Errorf("search request failed: %w", uerr.Err)
		}
		return nil, errors.New("search request failed")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search provider returned status %d", resp.StatusCode)
	}
	var out struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unreadable search response: %w", err)
	}
	results := make([]Result, 0, len(out.Results))
	for _, r := range out.Results {
		if len(results) == limit {
			break
		}
		results = append(results, Result{Title: r.Title, URL: r.URL, Snippet: r.Content})
	}
	return results, nil
}
