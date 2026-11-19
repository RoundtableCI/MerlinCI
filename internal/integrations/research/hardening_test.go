package research

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMatchRule(t *testing.T) {
	cases := []struct {
		pattern, target string
		want            bool
	}{
		{"/", "/anything", true},
		{"/private/", "/private/notes", true},
		{"/private/", "/public", false},
		{"/private", "/private", true},
		{"/*.pdf", "/files/report.pdf", true},
		{"/*.pdf", "/files/report.pdf?x=1", true},
		{"/*.pdf$", "/files/report.pdf", true},
		{"/*.pdf$", "/files/report.pdf?x=1", false},
		{"/$", "/", true},
		{"/$", "/post", false},
		{"/a*b*c", "/a-1-b-2-c-3", true},
		{"/a*b*c", "/a-1-c-2-b", false},
		{"/search?q=", "/search?q=models", true},
		{"/search?q=", "/search", false},
		{"/post$", "/post", true},
		{"/pos$", "/post", false},
		{"*", "/anything", true},
	}
	for _, tc := range cases {
		if got := matchRule(tc.pattern, tc.target); got != tc.want {
			t.Errorf("matchRule(%q, %q) = %v, want %v", tc.pattern, tc.target, got, tc.want)
		}
	}
}

// Ways a site's robots.txt could be read too generously.
func TestRobotsIsNotReadTooGenerously(t *testing.T) {
	cases := []struct {
		name, robots, path string
		allowed            bool
	}{
		{"allow for the home page only", "User-agent: *\nDisallow: /\nAllow: /$\n", "/post", false},
		{"allow for the home page only, home page", "User-agent: *\nDisallow: /\nAllow: /$\n", "/", true},
		{"rule on a query string", "User-agent: *\nDisallow: /post?q=\n", "/post?q=models", false},
		{"rule on a query string, other query", "User-agent: *\nDisallow: /post?q=\n", "/post?page=2", true},
		{"percent-encoded path", "User-agent: *\nDisallow: /private/\n", "/%70rivate/notes", false},
		{"percent-encoded rule", "User-agent: *\nDisallow: /%70rivate/\n", "/private/notes", false},
		{"byte-order mark before the first line", "\xef\xbb\xbfUser-agent: *\nDisallow: /\n", "/post", false},
		{"wildcard in the middle", "User-agent: *\nDisallow: /*/notes\n", "/private/notes", false},
		{"wildcard that does not match", "User-agent: *\nDisallow: /*/drafts\n", "/private/notes", true},
		{"end anchor", "User-agent: *\nDisallow: /post$\n", "/post", false},
		{"end anchor on a shorter path", "User-agent: *\nDisallow: /pos$\n", "/post", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := site(t, tc.robots)
			_, err := local().Fetch(ctx, srv.URL+tc.path)
			// The test site has no home page, so "allowed" means robots.txt
			// did not refuse, whatever the site then answered.
			refused := err != nil && strings.Contains(err.Error(), "robots.txt does not allow")
			if refused == tc.allowed {
				t.Fatalf("err = %v, want allowed by robots.txt = %v", err, tc.allowed)
			}
		})
	}
}

// A redirect is a new request. A page that robots.txt disallows must not be
// requested at all, whether the redirect stays on the site or leaves it.
func TestRedirectToADisallowedPageIsNotFollowed(t *testing.T) {
	hits := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "User-agent: *\nDisallow: /private/\n")
	})
	mux.HandleFunc("/open", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/private/secret", http.StatusFound)
	})
	mux.HandleFunc("/private/secret", func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = io.WriteString(w, "secret")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	page, err := local().Fetch(ctx, srv.URL+"/open")
	if err == nil || !strings.Contains(err.Error(), "robots.txt does not allow") {
		t.Fatalf("Fetch = %q, %v; want it refused by robots.txt", page.Text, err)
	}
	if hits != 0 {
		t.Errorf("the disallowed page was requested %d times, want 0", hits)
	}
}

// The check that matters most is the one made on the address a connection
// is actually about to be made to: it holds whatever the URL said and
// however the name resolved, including after a redirect.
func TestDialGuard(t *testing.T) {
	f := NewFetcher()
	for _, addr := range []string{"127.0.0.1:80", "10.0.0.5:443", "172.16.3.4:80", "192.168.1.1:80", "169.254.169.254:80", "100.64.0.1:80", "[::1]:80", "[fc00::1]:443", "[fe80::1]:80", "0.0.0.0:80"} {
		if err := f.control("tcp", addr, nil); err == nil {
			t.Errorf("a connection to %s was allowed", addr)
		}
	}
	for _, addr := range []string{"93.184.216.34:443", "[2606:4700:4700::1111]:443"} {
		if err := f.control("tcp", addr, nil); err != nil {
			t.Errorf("a connection to public address %s was refused: %v", addr, err)
		}
	}
	// It is the dial guard that refuses these, not a missing server or a
	// port rule: the error says so.
	for _, target := range []string{"http://127.0.0.1/", "http://localhost/", "http://[::1]/", "http://10.0.0.5/"} {
		_, err := f.Fetch(ctx, target)
		if err == nil || !strings.Contains(err.Error(), "is not a public address") {
			t.Errorf("Fetch(%q): err = %v; want it refused as not a public address", target, err)
		}
	}
}

func TestNULBytesAreRemovedFromPages(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	mux.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("be\x00fore and af\x00ter"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	page, err := local().Fetch(ctx, srv.URL+"/page")
	if err != nil || page.Text != "before and after" {
		t.Fatalf("text = %q, %v", page.Text, err)
	}
}
