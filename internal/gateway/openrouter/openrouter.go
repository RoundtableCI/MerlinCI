package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"time"

	"github.com/RoundtableCI/MerlinCI/pkg/provider"
)

// DefaultBaseURL is the OpenRouter API.
const DefaultBaseURL = "https://openrouter.ai/api/v1"

// Client is the OpenRouter model provider. It speaks OpenRouter's
// OpenAI-compatible chat completions API.
type Client struct {
	baseURL string
	key     func() (string, error)
	http    *http.Client
}

// New returns a client. key supplies the API key from the integration layer
// each time a request is made; the key is never stored on a request or
// returned in a response or an error.
func New(baseURL string, key func() (string, error), httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 2 * time.Minute}
	}
	return &Client{baseURL: baseURL, key: key, http: httpClient}
}

// Name implements provider.Provider.
func (c *Client) Name() string { return "openrouter" }

type wireMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type wireToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type wireTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

type wireRequest struct {
	Model     string        `json:"model"`
	Messages  []wireMessage `json:"messages"`
	Tools     []wireTool    `json:"tools,omitempty"`
	MaxTokens int           `json:"max_tokens"`
	Usage     struct {
		Include bool `json:"include"`
	} `json:"usage"`
}

type wireResponse struct {
	Choices []struct {
		Message      wireMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int          `json:"prompt_tokens"`
		CompletionTokens int          `json:"completion_tokens"`
		Cost             *json.Number `json:"cost"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete implements provider.Provider.
func (c *Client) Complete(ctx context.Context, req provider.Request) (provider.Response, error) {
	wire := wireRequest{Model: req.Model, MaxTokens: req.MaxOutputTokens}
	wire.Usage.Include = true
	for _, m := range req.Messages {
		wm := wireMessage{Role: string(m.Role), Content: m.Content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			var w wireToolCall
			w.ID, w.Type = tc.ID, "function"
			w.Function.Name, w.Function.Arguments = tc.Name, string(tc.Arguments)
			wm.ToolCalls = append(wm.ToolCalls, w)
		}
		wire.Messages = append(wire.Messages, wm)
	}
	for _, t := range req.Tools {
		var w wireTool
		w.Type = "function"
		w.Function.Name, w.Function.Description, w.Function.Parameters = t.Name, t.Description, t.InputSchema
		wire.Tools = append(wire.Tools, w)
	}
	body, err := json.Marshal(wire)
	if err != nil {
		return provider.Response{}, err
	}

	key, err := c.key()
	if err != nil {
		return provider.Response{}, fmt.Errorf("%w: %s", provider.ErrCredential, err.Error())
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return provider.Response{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+key)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.http.Do(httpReq)
	if err != nil {
		// A connection that was never made carried no request, so nothing
		// was billed. Any other transport failure might have been.
		var op *net.OpError
		if errors.As(err, &op) && op.Op == "dial" {
			return provider.Response{}, fmt.Errorf("%w: %s", provider.ErrNotBilled, err.Error())
		}
		return provider.Response{}, err
	}
	defer httpResp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(httpResp.Body, 16<<20))
	if err != nil {
		return provider.Response{}, err
	}

	var out wireResponse
	decodeErr := json.Unmarshal(raw, &out)
	if httpResp.StatusCode != http.StatusOK || out.Error != nil {
		msg := http.StatusText(httpResp.StatusCode)
		if out.Error != nil && out.Error.Message != "" {
			msg = out.Error.Message
		}
		if httpResp.StatusCode == http.StatusUnauthorized || httpResp.StatusCode == http.StatusForbidden {
			return provider.Response{}, fmt.Errorf("%w: status %d: %s", provider.ErrCredential, httpResp.StatusCode, msg)
		}
		if httpResp.StatusCode != http.StatusOK {
			// A request the provider refused is not billed.
			return provider.Response{}, fmt.Errorf("%w: status %d: %s", provider.ErrNotBilled, httpResp.StatusCode, msg)
		}
		return provider.Response{}, fmt.Errorf("status %d: %s", httpResp.StatusCode, msg)
	}
	if decodeErr != nil {
		return provider.Response{}, fmt.Errorf("unreadable response: %w", decodeErr)
	}
	if len(out.Choices) == 0 {
		return provider.Response{}, errors.New("response has no choices")
	}

	wm := out.Choices[0].Message
	msg := provider.Message{Role: provider.RoleAssistant, Content: wm.Content}
	for _, tc := range wm.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, provider.ToolCall{
			ID: tc.ID, Name: tc.Function.Name, Arguments: json.RawMessage(tc.Function.Arguments),
		})
	}
	usage := provider.Usage{InputTokens: out.Usage.PromptTokens, OutputTokens: out.Usage.CompletionTokens}
	if out.Usage.Cost != nil {
		// OpenRouter reports cost in US dollars. It is read as an exact
		// decimal and rounded up; a cost that cannot be read is left
		// unreported, and the gateway prices the call from its tokens.
		if micros, ok := dollarsToMicros(out.Usage.Cost.String()); ok {
			usage.CostMicros, usage.Currency = micros, "USD"
		}
	}
	return provider.Response{Message: msg, Usage: usage, Truncated: out.Choices[0].FinishReason == "length"}, nil
}

// dollarsToMicros converts a decimal number of dollars, as written in JSON,
// to millionths of a dollar, rounded up. No floating point is involved.
func dollarsToMicros(s string) (int64, bool) {
	r, ok := new(big.Rat).SetString(s)
	if !ok || r.Sign() < 0 {
		return 0, false
	}
	r.Mul(r, big.NewRat(1_000_000, 1))
	micros := new(big.Int).Quo(r.Num(), r.Denom())
	if new(big.Int).Rem(r.Num(), r.Denom()).Sign() != 0 {
		micros.Add(micros, big.NewInt(1))
	}
	if !micros.IsInt64() {
		return 0, false
	}
	return micros.Int64(), true
}
