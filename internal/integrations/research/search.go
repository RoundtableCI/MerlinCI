package research

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Result is one search result.
type Result struct {
	Title   string
	URL     string
	Snippet string
}

// Searcher is a web search provider. Implementations use the provider's
// official API and hold their own credentials.
type Searcher interface {
	Name() string
	Search(ctx context.Context, query string, limit int) ([]Result, error)
}

// BraveBaseURL is the Brave Search API.
const BraveBaseURL = "https://api.search.brave.com/res/v1"

// Brave searches the web through the Brave Search API.
type Brave struct {
	baseURL string
	key     func() (string, error)
	http    *http.Client
}

// NewBrave returns a Brave Search client. key supplies the API key from the
// integration layer each time a request is made.
func NewBrave(baseURL string, key func() (string, error), httpClient *http.Client) *Brave {
	if baseURL == "" {
		baseURL = BraveBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Brave{baseURL: baseURL, key: key, http: httpClient}
}

// Name implements Searcher.
func (b *Brave) Name() string { return "brave" }

// Search implements Searcher.
func (b *Brave) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	key, err := b.key()
	if err != nil {
		return nil, errors.New("no search credential is available")
	}
	q := url.Values{"q": {query}, "count": {strconv.Itoa(limit)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL+"/web/search?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", key)

	resp, err := b.http.Do(req)
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
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unreadable search response: %w", err)
	}
	results := make([]Result, 0, len(out.Web.Results))
	for _, r := range out.Web.Results {
		if len(results) == limit {
			break
		}
		results = append(results, Result{Title: r.Title, URL: r.URL, Snippet: r.Description})
	}
	return results, nil
}
