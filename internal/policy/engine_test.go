package policy

import (
	"testing"

	"github.com/RoundtableCI/MerlinCI/internal/config"
)

type counter int

func (c counter) CountToday(string, string) int { return int(c) }

func spec() config.PolicySpec {
	return config.PolicySpec{Default: "deny", Rules: []config.PolicyRule{
		{Action: "research.*", Tier: config.TierAutomatic},
		{Action: "research.paid", Tier: config.TierExplicit, Reason: "paid research"},
		{Action: "publish.post", Tier: config.TierConditional, Conditions: &config.Conditions{
			Channels: []string{"blog"}, RequireApprovedContent: true, MaxPerDay: 2,
		}},
		{Action: "campaign.delete", Tier: config.TierExplicit, Reason: "deleting campaigns"},
		{Action: "spend.*", Tier: config.TierAutomatic},
		{Action: "outreach.*", Tier: config.TierAutomatic},
		{Action: "budget.increase", Tier: config.TierAutomatic},
	}}
}

var all = []string{"research.search", "research.paid", "research.deep.crawl", "publish.post", "campaign.delete", "spend.ads", "outreach.dm", "budget.increase", "site.update"}

func TestDecide(t *testing.T) {
	cases := []struct {
		name    string
		req     Request
		count   int
		outcome Outcome
		tier    config.Tier
	}{
		{name: "automatic", req: Request{Action: "research.search"}, outcome: Allow, tier: config.TierAutomatic},
		{name: "prefix covers nested actions", req: Request{Action: "research.deep.crawl"}, outcome: Allow, tier: config.TierAutomatic},
		{name: "exact rule beats prefix rule", req: Request{Action: "research.paid"}, outcome: NeedsApproval, tier: config.TierExplicit},
		{name: "no rule matches", req: Request{Action: "site.update"}, outcome: Deny},
		{name: "explicit", req: Request{Action: "campaign.delete"}, outcome: NeedsApproval, tier: config.TierExplicit},
		{name: "explicit, approved", req: Request{Action: "campaign.delete", Approved: true}, outcome: Allow, tier: config.TierExplicit},

		{name: "conditional, all met", req: Request{Action: "publish.post", Channel: "blog", ContentApproved: true}, count: 1, outcome: Allow, tier: config.TierConditional},
		{name: "conditional, other channel", req: Request{Action: "publish.post", Channel: "x", ContentApproved: true}, outcome: NeedsApproval, tier: config.TierConditional},
		{name: "conditional, content not approved", req: Request{Action: "publish.post", Channel: "blog"}, outcome: NeedsApproval, tier: config.TierConditional},
		{name: "conditional, daily limit reached", req: Request{Action: "publish.post", Channel: "blog", ContentApproved: true}, count: 2, outcome: NeedsApproval, tier: config.TierConditional},
		{name: "conditional unmet, approved by a person", req: Request{Action: "publish.post", Channel: "x", Approved: true}, outcome: Allow, tier: config.TierConditional},

		// The floors hold although this policy marks all three automatic.
		{name: "ad spend", req: Request{Action: "spend.ads"}, outcome: NeedsApproval, tier: config.TierExplicit},
		{name: "cold outreach", req: Request{Action: "outreach.dm"}, outcome: NeedsApproval, tier: config.TierExplicit},
		{name: "budget increase", req: Request{Action: "budget.increase"}, outcome: NeedsApproval, tier: config.TierExplicit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.req.RolePermissions = all
			got := New(spec(), counter(tc.count)).Decide(tc.req)
			if got.Outcome != tc.outcome || got.Tier != tc.tier {
				t.Errorf("Decide = %s (tier %q, %s), want %s (tier %q)", got.Outcome, got.Tier, got.Reason, tc.outcome, tc.tier)
			}
		})
	}
}

func TestRolePermissionComesFirst(t *testing.T) {
	e := New(spec(), nil)
	got := e.Decide(Request{RolePermissions: []string{"research.search"}, Action: "campaign.delete", Approved: true})
	if got.Outcome != Deny {
		t.Errorf("an action outside the role's permissions was %s, even with approval", got.Outcome)
	}
	if got := e.Decide(Request{Action: "research.search"}); got.Outcome != Deny {
		t.Errorf("a role with no permissions was %s", got.Outcome)
	}
}

func TestEmptyPolicyDeniesEverything(t *testing.T) {
	e := New(config.PolicySpec{Default: "deny"}, nil)
	for _, action := range all {
		if got := e.Decide(Request{RolePermissions: all, Action: action, Approved: true}); got.Outcome != Deny {
			t.Errorf("%s was %s under an empty policy", action, got.Outcome)
		}
	}
}
