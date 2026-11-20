package research

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTavilySearch(t *testing.T) {
	var auth string
	var body struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
		Depth      string `json:"search_depth"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if r.Method != http.MethodPost || r.URL.Path != "/search" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		_, _ = io.WriteString(w, `{"query": "x", "results": [
			{"title": "Comparing models", "url": "https://example.com/a", "content": "A thread.", "score": 0.9},
			{"title": "Another", "url": "https://example.com/b", "content": "More.", "score": 0.8},
			{"title": "Third", "url": "https://example.com/c", "content": "Extra.", "score": 0.7}
		]}`)
	}))
	defer srv.Close()

	tv := NewTavily(srv.URL, func() (string, error) { return secret, nil }, srv.Client())
	results, err := tv.Search(ctx, "compare model outputs", 2)
	if err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer "+secret || body.Query != "compare model outputs" || body.MaxResults != 2 || body.Depth != "basic" {
		t.Errorf("request carried auth %q, body %+v", auth, body)
	}
	if len(results) != 2 || results[0] != (Result{Title: "Comparing models", URL: "https://example.com/a", Snippet: "A thread."}) {
		t.Errorf("results = %+v, want the first two", results)
	}
}

func TestTavilyErrorsNeverCarryTheKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	key := func() (string, error) { return secret, nil }

	_, err := NewTavily(srv.URL, key, srv.Client()).Search(ctx, "x y", 1)
	if err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), secret) {
		t.Fatalf("err = %v, want the status and no key", err)
	}
	srv.Close()
	_, err = NewTavily(srv.URL, key, nil).Search(ctx, "x y", 1)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("err = %v", err)
	}

	called := false
	srv2 := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv2.Close()
	noKey := func() (string, error) { return "", errors.New("not set") }
	if _, err := NewTavily(srv2.URL, noKey, srv2.Client()).Search(ctx, "x y", 1); err == nil || called {
		t.Fatalf("searched without a credential: err %v, called %v", err, called)
	}
}
