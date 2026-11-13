package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Document kinds of the open agent configuration format.
const (
	KindProfile     = "Profile"
	KindAudiences   = "Audiences"
	KindPolicy      = "Policy"
	KindBudget      = "Budget"
	KindRuntimeRole = "RuntimeRole"
	KindRole        = "Role"
	KindWorkflow    = "Workflow"
)

// Metadata names a document.
type Metadata struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Document is the envelope every configuration document shares.
type Document[S any] struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   Metadata `json:"metadata"`
	Spec       S        `json:"spec"`
}

type (
	Profile     = Document[ProfileSpec]
	Audiences   = Document[AudiencesSpec]
	Policy      = Document[PolicySpec]
	Budget      = Document[BudgetSpec]
	RuntimeRole = Document[RuntimeRoleSpec]
	Role        = Document[RoleSpec]
	Workflow    = Document[WorkflowSpec]
)

// ProfileSpec is the approved company and product profile agents work from
// (deck slide 7).
type ProfileSpec struct {
	Approval   Approval    `json:"approval"`
	Company    Company     `json:"company"`
	Products   []Product   `json:"products"`
	Claims     Claims      `json:"claims"`
	NorthStar  NorthStar   `json:"northStar"`
	Objectives []Objective `json:"objectives"`
	Channels   []Channel   `json:"channels"`
	Voice      *Voice      `json:"voice,omitempty"`
}

// Approval records whether a person has approved the profile. Nothing public
// is produced from a draft profile.
type Approval struct {
	Status     string `json:"status"`
	ApprovedBy string `json:"approvedBy,omitempty"`
	ApprovedOn string `json:"approvedOn,omitempty"`
}

// Approved reports whether the profile may be used for public content.
func (s ProfileSpec) Approved() bool { return s.Approval.Status == "approved" }

type Company struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

type Product struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Summary  string    `json:"summary"`
	Features []Feature `json:"features"`
}

type Feature struct {
	ID        string `json:"id"`
	Statement string `json:"statement"`
}

// Claims holds the claims rule: what may be said publicly, and what never may.
type Claims struct {
	Approved   []Claim  `json:"approved"`
	Prohibited []string `json:"prohibited"`
}

type Claim struct {
	ID        string `json:"id"`
	Statement string `json:"statement"`
	Evidence  string `json:"evidence"`
}

// NorthStar names the metric MerlinCI optimizes for (deck slide 4).
type NorthStar struct {
	Metric          string   `json:"metric"`
	ActivationEvent *string  `json:"activationEvent"`
	Decides         []string `json:"decides"`
	DiagnosticOnly  []string `json:"diagnosticOnly"`
}

type Objective struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type Channel struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Handle  string `json:"handle,omitempty"`
	Enabled bool   `json:"enabled"`
}

type Voice struct {
	Summary string   `json:"summary,omitempty"`
	Rules   []string `json:"rules,omitempty"`
}

// AudiencesSpec holds audience hypotheses and the competitor list.
type AudiencesSpec struct {
	Segments    []Segment    `json:"segments"`
	Competitors []Competitor `json:"competitors"`
}

type Segment struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type Competitor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Tier is one of the three tiers of autonomy (deck slide 11).
type Tier string

const (
	TierAutomatic   Tier = "automatic"
	TierConditional Tier = "conditional"
	TierExplicit    Tier = "explicit"
)

// PolicySpec maps actions to autonomy tiers. Unmatched actions are denied.
type PolicySpec struct {
	Default string       `json:"default"`
	Rules   []PolicyRule `json:"rules"`
}

type PolicyRule struct {
	Action     string      `json:"action"`
	Tier       Tier        `json:"tier"`
	Reason     string      `json:"reason,omitempty"`
	Conditions *Conditions `json:"conditions,omitempty"`
}

// Conditions is the channel, content and frequency policy of a conditional rule.
type Conditions struct {
	Channels               []string `json:"channels"`
	RequireApprovedContent bool     `json:"requireApprovedContent"`
	MaxPerDay              int      `json:"maxPerDay"`
}

// BudgetSpec caps spend before it is spent (deck slide 15).
type BudgetSpec struct {
	Currency        string    `json:"currency"`
	Limits          []Limit   `json:"limits"`
	AlertThresholds []float64 `json:"alertThresholds"`
	AtLimit         string    `json:"atLimit"`
	// ExchangeRates holds units of the budget currency per one unit of
	// another currency.
	ExchangeRates map[string]Rate `json:"exchangeRates,omitempty"`
	// UnitCosts holds what one call of a paid action costs, by action name.
	UnitCosts map[string]Amount `json:"unitCosts,omitempty"`
}

type Limit struct {
	Category string `json:"category"`
	Period   string `json:"period"`
	Amount   Amount `json:"amount"`
}

// RuntimeRoleSpec describes one of the runtime roles (deck slide 8).
type RuntimeRoleSpec struct {
	Responsibility string `json:"responsibility"`
}

// RoleSpec is a logical role: a configuration and a responsibility contract
// run by the one reusable runtime (deck slide 8).
type RoleSpec struct {
	RuntimeRole    string `json:"runtimeRole"`
	Enabled        bool   `json:"enabled"`
	Responsibility string `json:"responsibility"`
	Instructions   string `json:"instructions"`
	// Model names an allowlisted model. Empty means the default model.
	Model       string     `json:"model,omitempty"`
	Tools       []string   `json:"tools"`
	Permissions []string   `json:"permissions"`
	Output      RoleOutput `json:"output"`
	Evaluation  Evaluation `json:"evaluation"`
	Limits      RoleLimits `json:"limits"`
}

type RoleOutput struct {
	ArtifactKind string `json:"artifactKind"`
}

type Evaluation struct {
	Evaluators []string `json:"evaluators"`
}

type RoleLimits struct {
	MaxSteps int `json:"maxSteps"`
	// MaxOutputTokens bounds one model reply. Zero means the default.
	MaxOutputTokens int    `json:"maxOutputTokens,omitempty"`
	MaxCostPerTask  Amount `json:"maxCostPerTask"`
}

// WorkflowSpec is a workflow template (deck slides 9, 14).
type WorkflowSpec struct {
	Steps []Step `json:"steps"`
}

type Step struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Role        string   `json:"role,omitempty"`
	Evaluator   string   `json:"evaluator,omitempty"`
	Needs       []string `json:"needs,omitempty"`
	Produces    string   `json:"produces,omitempty"`
}

// Amount is money in minor units (hundredths) of the budget currency. It is
// written in configuration as a decimal string such as "150.00" so that no
// floating-point value ever represents money.
type Amount int64

// ParseAmount parses a decimal string with at most two decimal places.
func ParseAmount(s string) (Amount, error) {
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" || len(frac) > 2 || strings.ContainsAny(s, "+- ") {
		return 0, fmt.Errorf("invalid amount %q", s)
	}
	frac += strings.Repeat("0", 2-len(frac))
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q", s)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q", s)
	}
	// A trillion is far above any real budget and far below where the
	// ledger's millionths would overflow.
	const maxWhole = 1_000_000_000_000
	if w > maxWhole {
		return 0, fmt.Errorf("amount %q is too large", s)
	}
	return Amount(w*100 + f), nil
}

func (a Amount) String() string {
	return fmt.Sprintf("%d.%02d", int64(a)/100, int64(a)%100)
}

func (a Amount) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

func (a *Amount) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("amount must be a decimal string: %w", err)
	}
	v, err := ParseAmount(s)
	if err != nil {
		return err
	}
	*a = v
	return nil
}
