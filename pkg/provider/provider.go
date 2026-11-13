package provider

import (
	"context"
	"encoding/json"
	"errors"
)

// ErrCredential means the provider's credential is missing or was rejected.
// Trying the same request again cannot succeed until a person fixes it.
var ErrCredential = errors.New("the provider credential is missing or was rejected")

// ErrNotBilled marks a failure for which the provider certainly charged
// nothing: the request was refused, or never reached the provider. Any
// other failure may have been billed, and the budget treats it as spent.
var ErrNotBilled = errors.New("the provider did not bill this request")

// Role is who a message is from.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is one turn of a conversation.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content,omitempty"`
	// ToolCalls are the tool calls an assistant message proposes.
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	// ToolCallID names the call a tool message answers.
	ToolCallID string `json:"toolCallId,omitempty"`
}

// ToolCall is a tool call proposed by the model. It is a proposal only:
// MerlinCI decides whether it runs.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolSpec describes a tool to the model.
type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Request is one inference request.
type Request struct {
	Model           string
	Messages        []Message
	Tools           []ToolSpec
	MaxOutputTokens int
}

// Usage is what a request consumed.
type Usage struct {
	InputTokens  int
	OutputTokens int
	// CostMicros is the cost the provider reported, in millionths of
	// Currency. Currency is empty when the provider reported no cost.
	CostMicros int64
	Currency   string
}

// Response is the model's reply.
type Response struct {
	Message Message
	Usage   Usage
	// Truncated is true when the reply was cut off at the output limit.
	Truncated bool
}

// Provider is a model provider. Implementations hold their own credentials;
// nothing in a Request or Response carries a key.
type Provider interface {
	Name() string
	Complete(ctx context.Context, req Request) (Response, error)
}
