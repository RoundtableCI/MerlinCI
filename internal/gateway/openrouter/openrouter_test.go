package openrouter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RoundtableCI/MerlinCI/pkg/provider"
)

const secret = "sk-test-secret"

func key() (string, error) { return secret, nil }

func TestComplete(t *testing.T) {
	var got map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{
			"choices": [{"message": {"role": "assistant", "content": "",
				"tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "search", "arguments": "{\"query\":\"x\"}"}}]}}],
			"usage": {"prompt_tokens": 120, "completion_tokens": 30, "cost": 0.00042}
		}`)
	}))
	defer srv.Close()

	c := New(srv.URL, key, srv.Client())
	resp, err := c.Complete(context.Background(), provider.Request{
		Model:           "vendor/model",
		MaxOutputTokens: 256,
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: "be useful"},
			{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "call_0", Name: "search", Arguments: json.RawMessage(`{"query":"w"}`)}}},
			{Role: provider.RoleTool, ToolCallID: "call_0", Content: "result"},
		},
		Tools: []provider.ToolSpec{{Name: "search", Description: "search the web", InputSchema: json.RawMessage(`{"type":"object"}`)}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if auth != "Bearer "+secret {
		t.Errorf("Authorization = %q", auth)
	}
	if got["model"] != "vendor/model" || got["max_tokens"] != float64(256) {
		t.Errorf("request model %v, max_tokens %v", got["model"], got["max_tokens"])
	}
	msgs := got["messages"].([]any)
	if msgs[2].(map[string]any)["tool_call_id"] != "call_0" {
		t.Errorf("tool message = %v", msgs[2])
	}
	sentCall := msgs[1].(map[string]any)["tool_calls"].([]any)[0].(map[string]any)["function"].(map[string]any)
	if sentCall["arguments"] != `{"query":"w"}` {
		t.Errorf("tool call arguments were sent as %v, want a JSON string", sentCall["arguments"])
	}
	if got["tools"].([]any)[0].(map[string]any)["type"] != "function" {
		t.Errorf("tools = %v", got["tools"])
	}

	if len(resp.Message.ToolCalls) != 1 || resp.Message.ToolCalls[0].Name != "search" || string(resp.Message.ToolCalls[0].Arguments) != `{"query":"x"}` {
		t.Errorf("tool calls = %+v", resp.Message.ToolCalls)
	}
	want := provider.Usage{InputTokens: 120, OutputTokens: 30, CostMicros: 420, Currency: "USD"}
	if resp.Usage != want {
		t.Errorf("usage = %+v, want %+v", resp.Usage, want)
	}
}

func TestErrorsNeverCarryTheKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = io.WriteString(w, `{"error": {"message": "Insufficient credits"}}`)
	}))
	defer srv.Close()

	_, err := New(srv.URL, key, srv.Client()).Complete(context.Background(), provider.Request{Model: "m"})
	if err == nil || !strings.Contains(err.Error(), "402") || !strings.Contains(err.Error(), "Insufficient credits") {
		t.Fatalf("err = %v, want the status and the provider's message", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("the error contains the API key")
	}

	// An unreachable server: the transport error must not carry the key either.
	srv.Close()
	_, err = New(srv.URL, key, nil).Complete(context.Background(), provider.Request{Model: "m"})
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("err = %v", err)
	}
}

func TestMissingCredential(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv.Close()

	noKey := func() (string, error) { return "", errors.New("OPENROUTER_API_KEY is not set") }
	if _, err := New(srv.URL, noKey, srv.Client()).Complete(context.Background(), provider.Request{Model: "m"}); err == nil {
		t.Fatal("no error without a credential")
	}
	if called {
		t.Error("a request was sent without a credential")
	}
}

func TestMissingCostIsReportedAsMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"choices": [{"message": {"role": "assistant", "content": "hi"}}], "usage": {"prompt_tokens": 5, "completion_tokens": 1}}`)
	}))
	defer srv.Close()
	resp, err := New(srv.URL, key, srv.Client()).Complete(context.Background(), provider.Request{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage.Currency != "" || resp.Message.Content != "hi" {
		t.Errorf("usage = %+v, content = %q; want no currency when no cost is reported", resp.Usage, resp.Message.Content)
	}
}
