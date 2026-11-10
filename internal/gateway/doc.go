// Package gateway is the provider-neutral inference interface (deck slide 12).
//
// All model calls go through it (deck slide 10, step 03). It enforces the model
// allowlist (open decision 11), per-task budgets, routing and caching, and
// reports cost to internal/budget (deck slides 15, 17).
package gateway
