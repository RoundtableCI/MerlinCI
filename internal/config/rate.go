package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Rate is an exchange rate in ten-thousandths: units of the budget currency
// per one unit of another currency. It is written in configuration as a
// decimal string such as "90.00".
type Rate int64

// RateOne is the rate between a currency and itself.
const RateOne Rate = 10_000

// ParseRate parses a decimal string with at most four decimal places.
func ParseRate(s string) (Rate, error) {
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" || len(frac) > 4 || strings.ContainsAny(s, "+- ") {
		return 0, fmt.Errorf("invalid rate %q", s)
	}
	frac += strings.Repeat("0", 4-len(frac))
	w, err := strconv.ParseInt(whole, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid rate %q", s)
	}
	f, err := strconv.ParseInt(frac, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid rate %q", s)
	}
	return Rate(w*10_000 + f), nil
}

func (r Rate) String() string {
	return fmt.Sprintf("%d.%04d", int64(r)/10_000, int64(r)%10_000)
}

func (r Rate) MarshalJSON() ([]byte, error) { return json.Marshal(r.String()) }

func (r *Rate) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("rate must be a decimal string: %w", err)
	}
	v, err := ParseRate(s)
	if err != nil {
		return err
	}
	*r = v
	return nil
}
