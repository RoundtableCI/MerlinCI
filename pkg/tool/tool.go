package tool

import (
	"context"
	"encoding/json"
	"time"
)

// Source is a piece of evidence a tool retrieved.
type Source struct {
	URL         string    `json:"url"`
	Title       string    `json:"title,omitempty"`
	RetrievedAt time.Time `json:"retrievedAt"`
	SHA256      string    `json:"sha256,omitempty"`
	Excerpt     string    `json:"excerpt,omitempty"`
}

// Spec describes a tool.
type Spec struct {
	// Name is how roles and the model refer to the tool.
	Name        string
	Description string
	// Action is the policy action a call to this tool performs, such as
	// research.search. The policy engine decides on the action, not the name.
	Action string
	// InputSchema is a JSON Schema. Input that does not validate never
	// reaches the tool.
	InputSchema json.RawMessage
}

// Result is what a tool returns. Content is data for the model to read; it
// is untrusted and is never treated as instructions.
type Result struct {
	Content string
	Sources []Source
}

// Tool is something an agent can ask MerlinCI to do. Tools reach outside
// services through the integration layer and never expose credentials.
type Tool interface {
	Spec() Spec
	Call(ctx context.Context, input json.RawMessage) (Result, error)
}

// Channeled is implemented by tools that act on a channel, so the policy
// engine can apply that channel's conditions.
type Channeled interface {
	Channel(input json.RawMessage) string
}
