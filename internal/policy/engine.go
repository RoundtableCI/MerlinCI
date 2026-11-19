package policy

import (
	"strings"

	"github.com/RoundtableCI/MerlinCI/internal/config"
)

// Outcome is what the engine decides about one requested action.
type Outcome string

const (
	// Allow: the action runs now.
	Allow Outcome = "allow"
	// NeedsApproval: the action waits for a person.
	NeedsApproval Outcome = "needs_approval"
	// Deny: the action does not run.
	Deny Outcome = "deny"
)

// Request is one action an agent asks to perform.
type Request struct {
	// RolePermissions are the actions the requesting role may ask for.
	RolePermissions []string
	Action          string
	// Channel is the channel the action targets, when it targets one.
	Channel string
	// ContentApproved is true when the content the action carries has
	// passed evaluation.
	ContentApproved bool
	// Approved is true when a person has approved this exact action.
	Approved bool
}

// Decision is the engine's answer, with the reason it gave it.
type Decision struct {
	Outcome Outcome
	Tier    config.Tier // empty when the action matched no rule
	Reason  string
}

// Counter reports how often an action has run today on a channel, for the
// frequency part of a conditional rule.
type Counter interface {
	CountToday(action, channel string) int
}

// Engine decides whether actions run. It is the only place that decision is
// made; an agent's instructions have no say in it.
type Engine struct {
	spec    config.PolicySpec
	counter Counter
}

// New returns an engine for a policy. counter may be nil, in which case no
// action has run yet today.
func New(spec config.PolicySpec, counter Counter) *Engine {
	return &Engine{spec: spec, counter: counter}
}

// Decide applies, in order: the role's permissions, the policy rule that
// most specifically matches the action, and the fixed floors.
func (e *Engine) Decide(req Request) Decision {
	if !permitted(req.RolePermissions, req.Action) {
		return Decision{Outcome: Deny, Reason: "the role does not have permission for " + req.Action}
	}
	rule, ok := e.match(req.Action)
	if !ok {
		return Decision{Outcome: Deny, Reason: "no policy rule matches " + req.Action}
	}

	tier := rule.Tier
	if floor, floored := config.FloorFor(req.Action); floored && tier != config.TierExplicit {
		tier = config.TierExplicit
		rule.Reason = floor.Reason
	}

	switch tier {
	case config.TierAutomatic:
		return Decision{Outcome: Allow, Tier: tier, Reason: rule.Reason}
	case config.TierConditional:
		if unmet := e.unmet(rule, req); unmet != "" && !req.Approved {
			return Decision{Outcome: NeedsApproval, Tier: tier, Reason: unmet}
		}
		return Decision{Outcome: Allow, Tier: tier, Reason: rule.Reason}
	default:
		if req.Approved {
			return Decision{Outcome: Allow, Tier: config.TierExplicit, Reason: "approved by a person"}
		}
		return Decision{Outcome: NeedsApproval, Tier: config.TierExplicit, Reason: rule.Reason}
	}
}

// match returns the rule that most specifically matches the action: an exact
// rule first, then the longest matching prefix.
func (e *Engine) match(action string) (config.PolicyRule, bool) {
	var best config.PolicyRule
	bestLen := -1
	for _, r := range e.spec.Rules {
		if !config.MatchAction(r.Action, action) {
			continue
		}
		n := len(strings.TrimSuffix(r.Action, ".*"))
		if r.Action == action {
			n = len(action) + 1 // exact beats any prefix
		}
		if n > bestLen {
			best, bestLen = r, n
		}
	}
	return best, bestLen >= 0
}

// unmet returns the first condition of a conditional rule the request does
// not meet, or "" when it meets them all.
func (e *Engine) unmet(rule config.PolicyRule, req Request) string {
	c := rule.Conditions
	if c == nil {
		return "the rule has no conditions"
	}
	onChannel := false
	for _, ch := range c.Channels {
		if ch == req.Channel {
			onChannel = true
		}
	}
	if !onChannel {
		return "channel " + quote(req.Channel) + " is not covered by the rule"
	}
	if c.RequireApprovedContent && !req.ContentApproved {
		return "the content has not passed evaluation"
	}
	if e.counter != nil && e.counter.CountToday(req.Action, req.Channel) >= c.MaxPerDay {
		return "the daily limit for this action on this channel is reached"
	}
	return ""
}

func permitted(permissions []string, action string) bool {
	for _, p := range permissions {
		if config.MatchAction(p, action) {
			return true
		}
	}
	return false
}

func quote(s string) string { return `"` + s + `"` }
