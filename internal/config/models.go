package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// KindModels is the model allowlist document.
const KindModels = "Models"

// Models is the model allowlist document.
type Models = Document[ModelsSpec]

// ModelsSpec lists the only models MerlinCI may call (deck slide 15: model
// allowlist), with the prices used to cap each call before it is made.
type ModelsSpec struct {
	Provider string `json:"provider"`
	Currency string `json:"currency"`
	Default  string `json:"default"`
	// EvaluatorModel is the model evaluators use. Empty means Default.
	EvaluatorModel string  `json:"evaluatorModel,omitempty"`
	Models         []Model `json:"models"`
}

// Model is one allowed model and its price per million tokens.
type Model struct {
	ID                     string `json:"id"`
	InputPerMillionTokens  Price  `json:"inputPerMillionTokens"`
	OutputPerMillionTokens Price  `json:"outputPerMillionTokens"`
}

// Price is a price in millionths of a currency unit. It is written in
// configuration as a decimal string such as "0.10".
type Price int64

// ParsePrice parses a decimal string with at most six decimal places.
func ParsePrice(s string) (Price, error) {
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" || len(frac) > 6 || strings.ContainsAny(s, "+- ") {
		return 0, fmt.Errorf("invalid price %q", s)
	}
	frac += strings.Repeat("0", 6-len(frac))
	w, err := strconv.ParseInt(whole, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid price %q", s)
	}
	f, err := strconv.ParseInt(frac, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid price %q", s)
	}
	return Price(w*1_000_000 + f), nil
}

func (p Price) String() string {
	return fmt.Sprintf("%d.%06d", int64(p)/1_000_000, int64(p)%1_000_000)
}

func (p Price) MarshalJSON() ([]byte, error) { return json.Marshal(p.String()) }

func (p *Price) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("price must be a decimal string: %w", err)
	}
	v, err := ParsePrice(s)
	if err != nil {
		return err
	}
	*p = v
	return nil
}

func (b *Bundle) checkModels(report reporter) {
	subject := KindModels + "/" + b.Models.Metadata.Name
	spec := b.Models.Spec

	if d := firstDuplicate(spec.Models, func(m Model) string { return m.ID }); d != "" {
		report(Error, subject, "model %q is listed more than once", d)
	}
	listed := map[string]bool{}
	for _, m := range spec.Models {
		listed[m.ID] = true
		if m.InputPerMillionTokens <= 0 || m.OutputPerMillionTokens <= 0 {
			// A free price would make every reservation zero, and the
			// budget would cap nothing.
			report(Error, subject, "model %q needs prices greater than zero", m.ID)
		}
	}
	if spec.EvaluatorModel != "" && !listed[spec.EvaluatorModel] {
		report(Error, subject, "evaluator model %q is not in the list", spec.EvaluatorModel)
	}
	if !listed[spec.Default] {
		report(Error, subject, "default model %q is not in the list", spec.Default)
	}
	if b.Budget != nil && spec.Currency != b.Budget.Spec.Currency {
		if r, ok := b.Budget.Spec.ExchangeRates[spec.Currency]; !ok || r <= 0 {
			report(Error, subject, "models are priced in %s but the budget has no exchange rate for it", spec.Currency)
		}
	}
	for _, name := range sortedKeys(b.Roles) {
		if m := b.Roles[name].Spec.Model; m != "" && !listed[m] {
			report(Error, KindRole+"/"+name, "model %q is not on the allowlist", m)
		}
	}
}
