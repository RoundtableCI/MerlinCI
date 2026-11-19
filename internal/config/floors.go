package config

import "strings"

// Floor is an action that needs a person's approval every time, whatever the
// policy file says. Floors are fixed in code so that no configuration change,
// and no agent, can lower them.
type Floor struct {
	Pattern string
	Reason  string
}

// Floors come from the deck's absolute statements, not from its defaults:
// "No autonomous ad spend" and "Agents can never raise their own limit"
// (slide 15), and MerlinCI is not "an autonomous spam or mass-DM system"
// (slide 6). The other explicit-tier actions on slide 11 stay configurable.
var Floors = []Floor{
	{Pattern: "spend.*", Reason: "no autonomous ad spend"},
	{Pattern: "budget.increase.*", Reason: "agents can never raise their own limit"},
	{Pattern: "outreach.*", Reason: "no autonomous cold outreach or unsolicited DMs"},
}

// MatchAction reports whether an action pattern covers an action. A pattern is
// an exact action name, or a prefix ending in ".*" that covers the prefix
// itself and everything beneath it.
func MatchAction(pattern, action string) bool {
	if base, wild := strings.CutSuffix(pattern, ".*"); wild {
		return action == base || strings.HasPrefix(action, base+".")
	}
	return pattern == action
}

// FloorFor returns the floor covering an action, if any. The policy engine
// applies it after the policy file, so a floored action is never automatic or
// conditional.
func FloorFor(action string) (Floor, bool) {
	for _, f := range Floors {
		if MatchAction(f.Pattern, action) {
			return f, true
		}
	}
	return Floor{}, false
}

// floorOverlapping returns a floor that shares at least one action with the
// rule pattern.
func floorOverlapping(rulePattern string) (Floor, bool) {
	ruleBase, ruleWild := strings.CutSuffix(rulePattern, ".*")
	for _, f := range Floors {
		floorBase, floorWild := strings.CutSuffix(f.Pattern, ".*")
		if ruleBase == floorBase ||
			(ruleWild && strings.HasPrefix(floorBase, ruleBase+".")) ||
			(floorWild && strings.HasPrefix(ruleBase, floorBase+".")) {
			return f, true
		}
	}
	return Floor{}, false
}

// publicActions are the actions that put something in front of people outside
// the company. None of them runs while the product profile is a draft.
var publicActions = []string{"publish.*", "newsletter.*", "site.*", "reply.*", "outreach.*"}

// IsPublicAction reports whether an action is public-facing.
func IsPublicAction(action string) bool {
	for _, p := range publicActions {
		if MatchAction(p, action) {
			return true
		}
	}
	return false
}
