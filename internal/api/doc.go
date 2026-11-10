// Package api is the control plane HTTP API.
//
// It serves configuration, permissions, run and artifact inspection, and the
// approval queue for actions in the explicit-authorization tier. Every
// autonomous permission can be paused, inspected and revoked through it
// (deck slides 6, 10, 11).
package api
