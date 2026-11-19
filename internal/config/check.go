package config

import (
	"fmt"
	"sort"
)

// check validates the documents against each other: the rules a single
// schema cannot express.
func (b *Bundle) check() Problems {
	var ps Problems
	report := func(sev Severity, subject, format string, args ...any) {
		ps = append(ps, Problem{
			Severity: sev,
			File:     b.files[subject],
			Subject:  subject,
			Message:  fmt.Sprintf(format, args...),
		})
	}

	for _, kind := range []string{KindProfile, KindPolicy, KindBudget, KindModels} {
		missing := (kind == KindProfile && b.Profile == nil) ||
			(kind == KindPolicy && b.Policy == nil) ||
			(kind == KindBudget && b.Budget == nil) ||
			(kind == KindModels && b.Models == nil)
		if missing {
			ps = append(ps, Problem{Severity: Warning, Message: "no " + kind + " document: not a complete deployment"})
		}
	}

	if b.Profile != nil {
		b.checkProfile(report)
	}
	if b.Policy != nil {
		b.checkPolicy(report)
	}
	if b.Budget != nil {
		b.checkBudget(report)
	}
	if b.Models != nil {
		b.checkModels(report)
	}
	for _, name := range sortedKeys(b.Roles) {
		b.checkRole(name, report)
	}
	for _, name := range sortedKeys(b.Workflows) {
		b.checkWorkflow(name, report)
	}
	return ps
}

type reporter func(sev Severity, subject, format string, args ...any)

func (b *Bundle) checkProfile(report reporter) {
	subject := KindProfile + "/" + b.Profile.Metadata.Name
	spec := b.Profile.Spec

	if d := firstDuplicate(spec.Products, func(p Product) string { return p.ID }); d != "" {
		report(Error, subject, "duplicate product id %q", d)
	}
	for _, p := range spec.Products {
		if d := firstDuplicate(p.Features, func(f Feature) string { return f.ID }); d != "" {
			report(Error, subject, "product %q: duplicate feature id %q", p.ID, d)
		}
	}
	if d := firstDuplicate(spec.Claims.Approved, func(c Claim) string { return c.ID }); d != "" {
		report(Error, subject, "duplicate claim id %q", d)
	}
	if d := firstDuplicate(spec.Objectives, func(o Objective) string { return o.ID }); d != "" {
		report(Error, subject, "duplicate objective id %q", d)
	}
	if d := firstDuplicate(spec.Channels, func(c Channel) string { return c.ID }); d != "" {
		report(Error, subject, "duplicate channel id %q", d)
	}
	if !spec.Approved() {
		report(Warning, subject, "profile is a draft: nothing public can be produced from it until a person approves it")
	} else {
		for _, p := range spec.Products {
			if len(p.Features) == 0 {
				report(Error, subject, "approved profile: product %q has no features for agents to state", p.ID)
			}
		}
	}
	if spec.NorthStar.ActivationEvent == nil {
		report(Warning, subject, "northStar.activationEvent is not set: activation cannot be measured yet")
	}
}

func (b *Bundle) checkPolicy(report reporter) {
	subject := KindPolicy + "/" + b.Policy.Metadata.Name
	rules := b.Policy.Spec.Rules

	if d := firstDuplicate(rules, func(r PolicyRule) string { return r.Action }); d != "" {
		report(Error, subject, "more than one rule for action %q", d)
	}
	for _, r := range rules {
		if f, ok := floorOverlapping(r.Action); ok && r.Tier != TierExplicit {
			report(Error, subject, "rule %q: %s always needs a person's approval (%s); tier must be explicit", r.Action, f.Pattern, f.Reason)
		}
	}
	if b.Profile == nil {
		return
	}
	channels := map[string]bool{}
	for _, c := range b.Profile.Spec.Channels {
		channels[c.ID] = true
	}
	for _, r := range rules {
		if r.Conditions == nil {
			continue
		}
		for _, c := range r.Conditions.Channels {
			if !channels[c] {
				report(Error, subject, "rule %q: channel %q is not defined in the profile", r.Action, c)
			}
		}
	}
}

func (b *Bundle) checkBudget(report reporter) {
	subject := KindBudget + "/" + b.Budget.Metadata.Name
	spec := b.Budget.Spec

	byKey := map[[2]string]Amount{}
	for _, l := range spec.Limits {
		key := [2]string{l.Category, l.Period}
		if _, dup := byKey[key]; dup {
			report(Error, subject, "more than one %s limit per %s", l.Category, l.Period)
		}
		byKey[key] = l.Amount
		if l.Amount <= 0 {
			report(Error, subject, "%s limit per %s must be greater than zero", l.Category, l.Period)
		}
	}
	for _, category := range []string{"inference", "research"} {
		day, hasDay := byKey[[2]string{category, "day"}]
		month, hasMonth := byKey[[2]string{category, "month"}]
		if hasDay && hasMonth && day > month {
			report(Error, subject, "%s limit per day (%s) exceeds the limit per month (%s)", category, day, month)
		}
	}
	for _, cur := range sortedKeys(spec.ExchangeRates) {
		if cur == spec.Currency {
			report(Error, subject, "exchangeRates must not list the budget currency %s", cur)
		}
		if spec.ExchangeRates[cur] <= 0 {
			report(Error, subject, "exchange rate for %s must be greater than zero", cur)
		}
	}
	if !sort.Float64sAreSorted(spec.AlertThresholds) {
		report(Error, subject, "alertThresholds must be in ascending order")
	}
	if d := firstDuplicate(spec.AlertThresholds, func(t float64) string { return fmt.Sprint(t) }); d != "" {
		report(Error, subject, "duplicate alert threshold %s", d)
	}
}

func (b *Bundle) checkRole(name string, report reporter) {
	subject := KindRole + "/" + name
	spec := b.Roles[name].Spec

	if _, ok := b.RuntimeRoles[spec.RuntimeRole]; !ok {
		report(Error, subject, "runtime role %q is not defined", spec.RuntimeRole)
	}
	if spec.Limits.MaxCostPerTask <= 0 {
		report(Error, subject, "limits.maxCostPerTask must be greater than zero")
	}
	// Deck slide 13: do not enable a role merely because its prompt has been written.
	if spec.Enabled {
		if len(spec.Tools) > 0 && len(spec.Permissions) == 0 {
			report(Error, subject, "an enabled role with tools needs permissions for them")
		}
		if len(spec.Evaluation.Evaluators) == 0 {
			report(Error, subject, "an enabled role needs at least one evaluator")
		}
	}
}

func (b *Bundle) checkWorkflow(name string, report reporter) {
	subject := KindWorkflow + "/" + name
	steps := b.Workflows[name].Spec.Steps

	if d := firstDuplicate(steps, func(s Step) string { return s.ID }); d != "" {
		report(Error, subject, "duplicate step id %q", d)
		return
	}
	byID := map[string]Step{}
	for _, s := range steps {
		byID[s.ID] = s
	}
	for _, s := range steps {
		if s.Role != "" {
			_, logical := b.Roles[s.Role]
			_, runtime := b.RuntimeRoles[s.Role]
			if !logical && !runtime {
				report(Error, subject, "step %q: role %q is not defined", s.ID, s.Role)
			}
		}
		for _, n := range s.Needs {
			if _, ok := byID[n]; !ok {
				report(Error, subject, "step %q needs %q, which is not a step", s.ID, n)
			}
		}
	}

	// Depth-first search for a dependency cycle.
	const (
		unseen = iota
		visiting
		done
	)
	state := map[string]int{}
	var visit func(id string) bool
	visit = func(id string) bool {
		switch state[id] {
		case visiting:
			return true
		case done:
			return false
		}
		state[id] = visiting
		for _, n := range byID[id].Needs {
			if _, ok := byID[n]; ok && visit(n) {
				return true
			}
		}
		state[id] = done
		return false
	}
	for _, s := range steps {
		if visit(s.ID) {
			report(Error, subject, "steps form a dependency cycle through %q", s.ID)
			return
		}
	}
}

func firstDuplicate[T any](items []T, key func(T) string) string {
	seen := map[string]bool{}
	for _, it := range items {
		k := key(it)
		if seen[k] {
			return k
		}
		seen[k] = true
	}
	return ""
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
