package research

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/html"
)

// UserAgent identifies MerlinCI to the sites it reads.
const UserAgent = "MerlinCI/0.1 (research; respects robots.txt)"

// robotsToken is the name MerlinCI answers to in robots.txt.
const robotsToken = "merlinci"

const robotsPath = "/robots.txt"

var errRobots = errors.New("the site's robots.txt does not allow this page to be fetched")

// Page is a public web page, read as text.
type Page struct {
	URL         string // after redirects
	Title       string
	Text        string
	SHA256      string // of the body as received
	RetrievedAt time.Time
}

// Fetcher reads public web pages. It is built to be pointed at URLs chosen
// by a model, so it treats every URL as hostile:
//
//   - only http and https, on their standard ports, with no credentials in the URL;
//   - no address that is private, loopback or link-local, checked on the
//     address actually connected to, so DNS tricks and redirects cannot
//     reach the internal network;
//   - robots.txt is obeyed (deck slide 6: not a scraper that bypasses
//     platform restrictions);
//   - the response size and the time spent are bounded.
type Fetcher struct {
	client       *http.Client
	maxBytes     int64
	now          func() time.Time
	allowPrivate bool // tests only: lets the fetcher reach a local test server

	mu     sync.Mutex
	robots map[string]*robotsRules
}

// NewFetcher returns a Fetcher with safe defaults.
func NewFetcher() *Fetcher {
	f := &Fetcher{maxBytes: 2 << 20, now: time.Now, robots: map[string]*robotsRules{}}
	dialer := &net.Dialer{Timeout: 10 * time.Second, Control: f.control}
	f.client = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if err := f.checkURL(req.URL); err != nil {
				return err
			}
			// A redirect is a new request. It is checked against robots.txt
			// before it is made, on this site or another.
			if req.URL.Path == robotsPath {
				return nil
			}
			allowed, err := f.allowedByRobots(req.Context(), req.URL)
			if err != nil {
				return err
			}
			if !allowed {
				return errRobots
			}
			return nil
		},
	}
	return f
}

// control runs when a connection is about to be made, with the real address.
func (f *Fetcher) control(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || (!f.allowPrivate && !publicIP(ip)) {
		return fmt.Errorf("address %s is not a public address", host)
	}
	return nil
}

var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

func publicIP(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() || cgnat.Contains(ip))
}

func (f *Fetcher) checkURL(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http and https URLs can be fetched")
	}
	if u.User != nil {
		return fmt.Errorf("URLs with credentials cannot be fetched")
	}
	if u.Hostname() == "" {
		return fmt.Errorf("the URL has no host")
	}
	if p := u.Port(); p != "" && !f.allowPrivate && p != "80" && p != "443" {
		return fmt.Errorf("only the standard web ports can be fetched")
	}
	return nil
}

// Fetch reads one page.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (Page, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return Page{}, fmt.Errorf("not a valid URL")
	}
	if err := f.checkURL(u); err != nil {
		return Page{}, err
	}
	if allowed, err := f.allowedByRobots(ctx, u); err != nil {
		return Page{}, err
	} else if !allowed {
		return Page{}, errRobots
	}

	body, header, final, err := f.get(ctx, u.String())
	if err != nil {
		return Page{}, err
	}

	mediaType, _, _ := mime.ParseMediaType(header.Get("Content-Type"))
	sum := sha256.Sum256(body)
	page := Page{URL: final.String(), SHA256: hex.EncodeToString(sum[:]), RetrievedAt: f.now().UTC()}
	switch {
	case mediaType == "text/html" || mediaType == "application/xhtml+xml":
		page.Title, page.Text = htmlText(body)
	case strings.HasPrefix(mediaType, "text/") || mediaType == "application/json":
		page.Text = strings.ToValidUTF8(string(body), "")
	default:
		return Page{}, fmt.Errorf("the page is %q, which is not readable text", mediaType)
	}
	// A NUL byte is not text, and the database will not store one.
	page.Title = strings.ReplaceAll(page.Title, "\x00", "")
	page.Text = strings.ReplaceAll(page.Text, "\x00", "")
	return page, nil
}

func (f *Fetcher) get(ctx context.Context, target string) ([]byte, http.Header, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/html,text/plain;q=0.9,*/*;q=0.1")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not fetch the page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, resp.Request.URL, &statusError{resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBytes))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not read the page: %w", err)
	}
	return body, resp.Header, resp.Request.URL, nil
}

type statusError struct{ code int }

func (e *statusError) Error() string { return fmt.Sprintf("the site returned status %d", e.code) }

// htmlText returns the title and the visible text of an HTML document.
func htmlText(doc []byte) (title, text string) {
	z := html.NewTokenizer(bytes.NewReader(doc))
	var b strings.Builder
	skip, inTitle := 0, false
	for {
		switch z.Next() {
		case html.ErrorToken:
			return strings.TrimSpace(title), strings.TrimSpace(collapse(b.String()))
		case html.StartTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "script", "style", "noscript", "template", "svg", "head":
				if string(name) != "head" {
					skip++
				}
			case "title":
				inTitle = true
			case "p", "div", "br", "li", "tr", "h1", "h2", "h3", "h4", "h5", "h6", "section", "article", "blockquote":
				b.WriteByte('\n')
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "script", "style", "noscript", "template", "svg":
				if skip > 0 {
					skip--
				}
			case "title":
				inTitle = false
			case "p", "div", "li", "tr", "h1", "h2", "h3", "h4", "h5", "h6", "section", "article", "blockquote":
				b.WriteByte('\n')
			}
		case html.TextToken:
			switch {
			case inTitle:
				title += string(z.Text())
			case skip == 0:
				b.Write(z.Text())
				b.WriteByte(' ')
			}
		}
	}
}

// collapse squeezes runs of spaces and blank lines.
func collapse(s string) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// NewFetcherWithTransport returns a Fetcher that sends its requests through
// the caller's transport instead of the guarded one. The address checks are
// made by the guarded transport, so they do not apply here: this is for
// tests that stand a local server in for the public web, and nothing else.
func NewFetcherWithTransport(rt http.RoundTripper) *Fetcher {
	f := NewFetcher()
	f.client.Transport = rt
	return f
}
