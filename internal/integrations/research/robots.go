package research

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// robotsRules are the robots.txt rules that apply to MerlinCI on one site.
type robotsRules struct {
	allow    []string
	disallow []string
}

// allowed applies the most specific matching rule, which is the longest
// pattern; Allow wins a tie. A target no rule matches is allowed.
func (r *robotsRules) allowed(target string) bool {
	best, ok := -1, true
	for _, p := range r.disallow {
		if p != "" && matchRule(p, target) && len(p) > best {
			best, ok = len(p), false
		}
	}
	for _, p := range r.allow {
		if p != "" && matchRule(p, target) && len(p) >= best {
			best, ok = len(p), true
		}
	}
	return ok
}

// matchRule reports whether a robots.txt pattern matches a target. A pattern
// matches from the start of the target; "*" stands for any run of
// characters and a final "$" anchors the pattern to the end.
func matchRule(pattern, target string) bool {
	anchored := strings.HasSuffix(pattern, "$")
	if anchored {
		pattern = pattern[:len(pattern)-1]
	}
	parts := strings.Split(pattern, "*")
	if !strings.HasPrefix(target, parts[0]) {
		return false
	}
	pos := len(parts[0])
	for i := 1; i < len(parts); i++ {
		part := parts[i]
		if anchored && i == len(parts)-1 {
			return len(target)-len(part) >= pos && strings.HasSuffix(target, part)
		}
		idx := strings.Index(target[pos:], part)
		if idx < 0 {
			return false
		}
		pos += idx + len(part)
	}
	return !anchored || pos == len(target)
}

// robotsTarget is the form of a URL that rules are matched against: its
// path with percent-encoding decoded, so that "/%70rivate/" and "/private/"
// are the same place, followed by its query.
func robotsTarget(u *url.URL) string {
	target := u.Path
	if target == "" {
		target = "/"
	}
	if u.RawQuery != "" {
		target += "?" + u.RawQuery
	}
	return target
}

// parseRobots reads a robots.txt. Rules addressed to MerlinCI by name are
// used when present; otherwise the rules for every crawler ("*") apply.
func parseRobots(body []byte) *robotsRules {
	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf")) // a byte-order mark is not part of the first line
	groups := map[string]*robotsRules{}
	var agents []string
	inRules := false
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		field, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		field, value = strings.ToLower(strings.TrimSpace(field)), strings.TrimSpace(value)
		switch field {
		case "user-agent":
			if inRules {
				agents, inRules = nil, false
			}
			agents = append(agents, strings.ToLower(value))
		case "allow", "disallow":
			inRules = true
			// Decode the pattern the same way targets are decoded.
			if decoded, err := url.PathUnescape(value); err == nil {
				value = decoded
			}
			for _, a := range agents {
				g := groups[a]
				if g == nil {
					g = &robotsRules{}
					groups[a] = g
				}
				if field == "allow" {
					g.allow = append(g.allow, value)
				} else {
					g.disallow = append(g.disallow, value)
				}
			}
		}
	}
	if g, ok := groups[robotsToken]; ok {
		return g
	}
	if g, ok := groups["*"]; ok {
		return g
	}
	return &robotsRules{}
}

// allowedByRobots reports whether the site lets MerlinCI fetch the URL. The
// rules of a site are read once and kept for the life of the Fetcher.
func (f *Fetcher) allowedByRobots(ctx context.Context, u *url.URL) (bool, error) {
	site := u.Scheme + "://" + u.Host
	f.mu.Lock()
	rules, cached := f.robots[site]
	f.mu.Unlock()

	if !cached {
		body, _, _, err := f.get(ctx, site+robotsPath)
		var status *statusError
		switch {
		case err == nil:
			rules = parseRobots(body)
		case errors.As(err, &status) && status.code >= 400 && status.code < 500:
			rules = &robotsRules{} // no robots.txt: nothing is disallowed
		default:
			// The rules could not be read, so do not guess that
			// fetching is allowed.
			return false, fmt.Errorf("could not read the site's robots.txt: %w", err)
		}
		f.mu.Lock()
		f.robots[site] = rules
		f.mu.Unlock()
	}
	return rules.allowed(robotsTarget(u)), nil
}
