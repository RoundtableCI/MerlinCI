package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The configuration shipped in the repository must always be valid.
func TestShippedConfigIsValid(t *testing.T) {
	b, ps := Load("../../configs")
	for _, p := range ps {
		if p.Severity == Error {
			t.Error(p)
		}
	}
	if b.Profile == nil || b.Policy == nil || b.Budget == nil || b.Audiences == nil {
		t.Fatal("expected a profile, audiences, policy and budget")
	}
	if got := len(b.RuntimeRoles); got != 8 {
		t.Errorf("runtime roles = %d, want 8 (deck slide 8)", got)
	}
	if mr := b.Roles["market-researcher"].Spec; !mr.Enabled || len(mr.Tools) != 2 || len(mr.Evaluation.Evaluators) != 1 {
		t.Errorf("market-researcher = enabled %v, tools %v, evaluators %v; want enabled with 2 tools and 1 evaluator", mr.Enabled, mr.Tools, mr.Evaluation.Evaluators)
	}
	if b.Models == nil || b.Models.Spec.Default == "" {
		t.Error("expected a model allowlist with a default")
	}
	if got := b.Budget.Spec.Limits[0].Amount; got != 15000 {
		t.Errorf("daily inference cap = %d minor units, want 15000", got)
	}
}

func TestDigestIsStableAndSensitive(t *testing.T) {
	a, _ := Load("../../configs")
	b, _ := Load("../../configs")
	if a.Digest != b.Digest {
		t.Fatal("digest differs between two loads of the same configuration")
	}
	dir := write(t, map[string]string{"r.yaml": runtimeRole("cmo")})
	c, _ := Load(dir)
	if c.Digest == a.Digest {
		t.Fatal("digest did not change with the configuration")
	}
}

func TestParseAmount(t *testing.T) {
	good := map[string]Amount{"0": 0, "150": 15000, "150.5": 15050, "150.05": 15005, "3000.00": 300000}
	for in, want := range good {
		got, err := ParseAmount(in)
		if err != nil || got != want {
			t.Errorf("ParseAmount(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", ".5", "1.234", "-1", "+1", "1e3", "abc", "99999999999999999999", "1000000000000001", "1000000000000000.00"} {
		if _, err := ParseAmount(in); err == nil {
			t.Errorf("ParseAmount(%q) succeeded, want an error", in)
		}
	}
	if s := Amount(15005).String(); s != "150.05" {
		t.Errorf("String() = %q, want 150.05", s)
	}
}

func TestInvalidConfigIsRejected(t *testing.T) {
	const (
		policyHead = "apiVersion: merlinci/v1alpha1\nkind: Policy\nmetadata: {name: p}\nspec:\n"
		budgetHead = "apiVersion: merlinci/v1alpha1\nkind: Budget\nmetadata: {name: b}\nspec:\n  currency: INR\n  atLimit: stop\n"
		flowHead   = "apiVersion: merlinci/v1alpha1\nkind: Workflow\nmetadata: {name: w}\nspec:\n  steps:\n"
	)
	cases := []struct {
		name  string
		files map[string]string
		want  string // substring of an expected error
	}{
		{
			name:  "unknown kind",
			files: map[string]string{"a.yaml": "apiVersion: merlinci/v1alpha1\nkind: Agent\nmetadata: {name: a}\nspec: {}\n"},
			want:  `unknown kind "Agent"`,
		},
		{
			name:  "wrong apiVersion",
			files: map[string]string{"a.yaml": strings.Replace(runtimeRole("cmo"), "v1alpha1", "v9", 1)},
			want:  "apiVersion",
		},
		{
			name:  "unknown field",
			files: map[string]string{"a.yaml": runtimeRole("cmo") + "  model: fast\n"},
			want:  "additional properties",
		},
		{
			name:  "duplicate document",
			files: map[string]string{"a.yaml": runtimeRole("cmo"), "b.yaml": runtimeRole("cmo")},
			want:  "already defined in",
		},
		{
			name:  "policy default must be deny",
			files: map[string]string{"a.yaml": policyHead + "  default: allow\n  rules: []\n"},
			want:  "default",
		},
		{
			name:  "unknown tier",
			files: map[string]string{"a.yaml": policyHead + "  default: deny\n  rules:\n    - {action: research.search, tier: always}\n"},
			want:  "tier",
		},
		{
			name:  "conditional tier needs conditions",
			files: map[string]string{"a.yaml": policyHead + "  default: deny\n  rules:\n    - {action: publish.post, tier: conditional}\n"},
			want:  "conditions",
		},
		{
			name:  "two rules for one action",
			files: map[string]string{"a.yaml": policyHead + "  default: deny\n  rules:\n    - {action: research.*, tier: automatic}\n    - {action: research.*, tier: explicit}\n"},
			want:  "more than one rule",
		},
		{
			name:  "ad spend below the floor",
			files: map[string]string{"a.yaml": policyHead + "  default: deny\n  rules:\n    - {action: spend.ads, tier: automatic}\n"},
			want:  "no autonomous ad spend",
		},
		{
			name:  "wildcard reaching under a floor",
			files: map[string]string{"a.yaml": policyHead + "  default: deny\n  rules:\n    - {action: outreach.*, tier: conditional, conditions: {channels: [x], requireApprovedContent: true, maxPerDay: 1}}\n"},
			want:  "tier must be explicit",
		},
		{
			name:  "budget increase below the floor",
			files: map[string]string{"a.yaml": policyHead + "  default: deny\n  rules:\n    - {action: budget.increase, tier: automatic}\n"},
			want:  "agents can never raise their own limit",
		},
		{
			name:  "profile without approval status",
			files: map[string]string{"a.yaml": profile("")},
			want:  "approval",
		},
		{
			name:  "approved profile without approver",
			files: map[string]string{"a.yaml": profile("  approval: {status: approved}\n")},
			want:  "approvedBy",
		},
		{
			name:  "approved profile with nothing to say",
			files: map[string]string{"a.yaml": profile("  approval: {status: approved, approvedBy: someone, approvedOn: \"2026-11-13\"}\n")},
			want:  "has no features",
		},
		{
			name:  "money as a number",
			files: map[string]string{"a.yaml": budgetHead + "  alertThresholds: [0.7]\n  limits:\n    - {category: inference, period: day, amount: 150}\n"},
			want:  "amount",
		},
		{
			name:  "daily limit above monthly limit",
			files: map[string]string{"a.yaml": budgetHead + "  alertThresholds: [0.7]\n  limits:\n    - {category: inference, period: day, amount: \"500\"}\n    - {category: inference, period: month, amount: \"100\"}\n"},
			want:  "exceeds the limit per month",
		},
		{
			name:  "alert thresholds out of order",
			files: map[string]string{"a.yaml": budgetHead + "  alertThresholds: [0.9, 0.7]\n  limits:\n    - {category: inference, period: day, amount: \"150\"}\n"},
			want:  "ascending",
		},
		{
			name:  "role bound to a missing runtime role",
			files: map[string]string{"a.yaml": role("researcher", "intelligence", false, "")},
			want:  `runtime role "intelligence" is not defined`,
		},
		{
			name: "enabled role without evaluation",
			files: map[string]string{
				"a.yaml": runtimeRole("intelligence"),
				"b.yaml": role("researcher", "intelligence", true, ""),
			},
			want: "an enabled role needs at least one evaluator",
		},
		{
			name:  "workflow step with an undefined role",
			files: map[string]string{"a.yaml": flowHead + "    - {id: one, description: d, role: nobody}\n"},
			want:  `role "nobody" is not defined`,
		},
		{
			name:  "workflow step with both role and evaluator",
			files: map[string]string{"a.yaml": runtimeRole("cmo") + "---\n" + flowHead + "    - {id: one, description: d, role: cmo, evaluator: brand-guardian}\n"},
			want:  "oneOf",
		},
		{
			name: "workflow cycle",
			files: map[string]string{
				"a.yaml": runtimeRole("cmo"),
				"b.yaml": flowHead + "    - {id: one, description: d, role: cmo, needs: [two]}\n    - {id: two, description: d, role: cmo, needs: [one]}\n",
			},
			want: "dependency cycle",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ps := Load(write(t, tc.files))
			if !ps.HasErrors() {
				t.Fatalf("configuration was accepted; want an error containing %q", tc.want)
			}
			for _, p := range ps {
				if p.Severity == Error && strings.Contains(p.String(), tc.want) {
					return
				}
			}
			t.Fatalf("no error contains %q; got:\n%v", tc.want, ps)
		})
	}
}

func TestFloors(t *testing.T) {
	for _, action := range []string{"spend", "spend.ads", "spend.ads.search", "budget.increase", "budget.increase.daily", "outreach.dm"} {
		if _, ok := FloorFor(action); !ok {
			t.Errorf("FloorFor(%q) found no floor", action)
		}
	}
	for _, action := range []string{"research.search", "spending.report", "budget.report", "publish.post"} {
		if f, ok := FloorFor(action); ok {
			t.Errorf("FloorFor(%q) = %q, want none", action, f.Pattern)
		}
	}
}

func TestDraftProfileIsNotApproved(t *testing.T) {
	b, ps := Load("../../configs")
	if b.Profile.Spec.Approved() {
		t.Fatal("the shipped profile has placeholder content and must stay a draft")
	}
	for _, p := range ps {
		if p.Severity == Warning && strings.Contains(p.Message, "profile is a draft") {
			return
		}
	}
	t.Fatal("no warning that the profile is a draft")
}

func profile(approval string) string {
	return "apiVersion: merlinci/v1alpha1\nkind: Profile\nmetadata: {name: p}\nspec:\n" + approval +
		"  company: {name: C}\n  products:\n    - {id: p, name: P, summary: s, features: []}\n" +
		"  claims: {approved: [], prohibited: []}\n" +
		"  northStar: {metric: m, activationEvent: e, decides: [], diagnosticOnly: []}\n" +
		"  objectives:\n    - {id: o, description: d}\n  channels: []\n"
}

func runtimeRole(name string) string {
	return "apiVersion: merlinci/v1alpha1\nkind: RuntimeRole\nmetadata: {name: " + name + "}\nspec:\n  responsibility: r\n"
}

func role(name, runtime string, enabled bool, tools string) string {
	e := "false"
	if enabled {
		e = "true"
	}
	return "apiVersion: merlinci/v1alpha1\nkind: Role\nmetadata: {name: " + name + "}\nspec:\n" +
		"  runtimeRole: " + runtime + "\n  enabled: " + e + "\n  responsibility: r\n  instructions: i\n" +
		"  tools: [" + tools + "]\n  permissions: []\n  output: {artifactKind: note}\n" +
		"  evaluation: {evaluators: []}\n  limits: {maxSteps: 1, maxCostPerTask: \"1.00\"}\n"
}

func write(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
