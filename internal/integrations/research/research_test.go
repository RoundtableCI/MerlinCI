package research

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var ctx = context.Background()

const secret = "brave-test-secret"

func TestBraveSearch(t *testing.T) {
	var token, query, count string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token = r.Header.Get("X-Subscription-Token")
		query, count = r.URL.Query().Get("q"), r.URL.Query().Get("count")
		if r.URL.Path != "/web/search" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"web": {"results": [
			{"title": "Comparing models", "url": "https://example.com/a", "description": "A thread."},
			{"title": "Another", "url": "https://example.com/b", "description": "More."},
			{"title": "Third", "url": "https://example.com/c", "description": "Extra."}
		]}}`)
	}))
	defer srv.Close()

	b := NewBrave(srv.URL, func() (string, error) { return secret, nil }, srv.Client())
	results, err := b.Search(ctx, "compare model outputs", 2)
	if err != nil {
		t.Fatal(err)
	}
	if token != secret || query != "compare model outputs" || count != "2" {
		t.Errorf("request carried token %q, q %q, count %q", token, query, count)
	}
	if len(results) != 2 || results[0] != (Result{Title: "Comparing models", URL: "https://example.com/a", Snippet: "A thread."}) {
		t.Errorf("results = %+v, want the first two", results)
	}
}

func TestBraveErrorsNeverCarryTheKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	key := func() (string, error) { return secret, nil }

	_, err := NewBrave(srv.URL, key, srv.Client()).Search(ctx, "x y", 1)
	if err == nil || !strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), secret) {
		t.Fatalf("err = %v, want the status and no key", err)
	}
	srv.Close()
	_, err = NewBrave(srv.URL, key, nil).Search(ctx, "x y", 1)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("err = %v", err)
	}

	called := false
	srv2 := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv2.Close()
	noKey := func() (string, error) { return "", errors.New("not set") }
	if _, err := NewBrave(srv2.URL, noKey, srv2.Client()).Search(ctx, "x y", 1); err == nil || called {
		t.Fatalf("searched without a credential: err %v, called %v", err, called)
	}
}

// site serves a small website for the fetcher.
func site(t *testing.T, robots string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		if robots == "404" {
			http.NotFound(w, r)
			return
		}
		if robots == "500" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, robots)
	})
	mux.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, "MerlinCI/") {
			t.Errorf("User-Agent = %q", ua)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<html><head><title> Comparing models </title><style>p{color:red}</style></head>
			<body><script>var secret = "ignore me";</script>
			<h1>How do you compare models?</h1><p>I run the   same prompt on three.</p>
			<ul><li>One</li><li>Two</li></ul></body></html>`)
	})
	mux.HandleFunc("/private/notes", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "private")
	})
	mux.HandleFunc("/big", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, strings.Repeat("a", 5<<20))
	})
	mux.HandleFunc("/image", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x89, 'P', 'N', 'G'})
	})
	mux.HandleFunc("/moved", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/post", http.StatusFound)
	})
	mux.HandleFunc("/to-file", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "file:///etc/passwd", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// local returns a fetcher allowed to reach the local test server.
func local() *Fetcher {
	f := NewFetcher()
	f.allowPrivate = true
	return f
}

func TestFetchReadsAPageAsText(t *testing.T) {
	srv := site(t, "User-agent: *\nDisallow: /private/\n")
	page, err := local().Fetch(ctx, srv.URL+"/post")
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "Comparing models" {
		t.Errorf("title = %q", page.Title)
	}
	want := "How do you compare models?\nI run the same prompt on three.\nOne\nTwo"
	if page.Text != want {
		t.Errorf("text = %q, want %q", page.Text, want)
	}
	if strings.Contains(page.Text, "ignore me") || strings.Contains(page.Text, "color:red") {
		t.Error("script or style content leaked into the text")
	}
	if len(page.SHA256) != 64 || page.RetrievedAt.IsZero() || page.URL != srv.URL+"/post" {
		t.Errorf("provenance is incomplete: %+v", page)
	}
}

func TestFetchFollowsRedirectsAndReportsTheFinalURL(t *testing.T) {
	srv := site(t, "404")
	page, err := local().Fetch(ctx, srv.URL+"/moved")
	if err != nil || page.URL != srv.URL+"/post" {
		t.Fatalf("page URL = %q, %v; want the redirect target", page.URL, err)
	}
}

func TestFetchObeysRobots(t *testing.T) {
	cases := []struct {
		name, robots, path string
		allowed            bool
	}{
		{"disallowed path", "User-agent: *\nDisallow: /private/\n", "/private/notes", false},
		{"allowed path", "User-agent: *\nDisallow: /private/\n", "/post", true},
		{"everything disallowed", "User-agent: *\nDisallow: /\n", "/post", false},
		{"allow overrides a shorter disallow", "User-agent: *\nDisallow: /private/\nAllow: /private/notes\n", "/private/notes", true},
		{"rules for another crawler do not apply", "User-agent: otherbot\nDisallow: /\n", "/post", true},
		{"rules naming MerlinCI win over *", "User-agent: *\nAllow: /\n\nUser-agent: MerlinCI\nDisallow: /post\n", "/post", false},
		{"wildcard pattern is treated as its prefix", "User-agent: *\nDisallow: /priv*notes\n", "/private/notes", false},
		{"no robots.txt", "404", "/post", true},
		{"robots.txt unreadable", "500", "/post", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := site(t, tc.robots)
			_, err := local().Fetch(ctx, srv.URL+tc.path)
			if (err == nil) != tc.allowed {
				t.Fatalf("err = %v, want allowed = %v", err, tc.allowed)
			}
		})
	}
}

func TestFetchBoundsAndRejects(t *testing.T) {
	srv := site(t, "404")
	f := local()

	page, err := f.Fetch(ctx, srv.URL+"/big")
	if err != nil || len(page.Text) != 2<<20 {
		t.Errorf("a 5 MiB page gave %d bytes, %v; want it cut at 2 MiB", len(page.Text), err)
	}
	if _, err := f.Fetch(ctx, srv.URL+"/image"); err == nil || !strings.Contains(err.Error(), "not readable text") {
		t.Errorf("an image was accepted: %v", err)
	}
	if _, err := f.Fetch(ctx, srv.URL+"/missing"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("a missing page: %v", err)
	}
	if _, err := f.Fetch(ctx, srv.URL+"/to-file"); err == nil {
		t.Error("a redirect to a file: URL was followed")
	}
}

// The fetcher is pointed at URLs chosen by a model. It must not be usable
// to reach the machine it runs on or the network around it. This test
// shows that each target is refused; TestDialGuard shows that the refusal
// comes from the check on the connected address, not from luck.
func TestFetchRefusesNonPublicTargets(t *testing.T) {
	srv := site(t, "404") // a real server on 127.0.0.1
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	f := NewFetcher() // production settings

	targets := []string{
		srv.URL + "/post",                          // loopback, non-standard port
		"http://127.0.0.1/",                        // loopback
		"http://localhost/",                        // resolves to loopback
		"http://[::1]/",                            // IPv6 loopback
		"http://10.0.0.5/",                         // private
		"http://192.168.1.1/",                      // private
		"http://169.254.169.254/latest/meta-data/", // cloud metadata service
		"http://100.64.0.1/",                       // carrier-grade NAT
		"http://0.0.0.0/",                          // unspecified
		"http://127.0.0.1:" + port + "/post",       // non-standard port
		"file:///etc/passwd",
		"ftp://example.com/file",
		"gopher://example.com/",
		"http://user:pass@example.com/",
		"http:///nohost",
		"not a url",
	}
	for _, target := range targets {
		if page, err := f.Fetch(ctx, target); err == nil {
			t.Errorf("Fetch(%q) succeeded and returned %q", target, page.Text)
		}
	}
}

func TestPublicIP(t *testing.T) {
	for _, ip := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !publicIP(net.ParseIP(ip)) {
			t.Errorf("%s was treated as non-public", ip)
		}
	}
	for _, ip := range []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.0.1", "169.254.1.1", "100.64.0.1", "::1", "fc00::1", "fe80::1", "0.0.0.0", "224.0.0.1"} {
		if publicIP(net.ParseIP(ip)) {
			t.Errorf("%s was treated as public", ip)
		}
	}
}
