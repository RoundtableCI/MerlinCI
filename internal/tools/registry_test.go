package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/RoundtableCI/MerlinCI/pkg/tool"
)

type stub struct{ spec tool.Spec }

func (s stub) Spec() tool.Spec { return s.spec }

func (s stub) Call(context.Context, json.RawMessage) (tool.Result, error) { return tool.Result{}, nil }

const schema = `{"type":"object","required":["query"],"additionalProperties":false,"properties":{"query":{"type":"string","maxLength":200}}}`

func search() stub {
	return stub{tool.Spec{Name: "search", Action: "research.search", InputSchema: json.RawMessage(schema)}}
}

func TestRegister(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(search()); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(search()); err == nil {
		t.Error("a second tool with the same name was registered")
	}
	if err := r.Register(stub{tool.Spec{Name: "nameless-action", InputSchema: json.RawMessage(schema)}}); err == nil {
		t.Error("a tool with no policy action was registered")
	}
	if err := r.Register(stub{tool.Spec{Name: "bad", Action: "a.b", InputSchema: json.RawMessage(`{"type": 7}`)}}); err == nil {
		t.Error("a tool with an invalid input schema was registered")
	}
	if err := r.Register(stub{tool.Spec{Name: "none", Action: "a.b"}}); err == nil {
		t.Error("a tool with no input schema was registered")
	}
	if names := r.Names(); len(names) != 1 || names[0] != "search" {
		t.Errorf("Names() = %v", names)
	}
}

func TestValidateInput(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(search()); err != nil {
		t.Fatal(err)
	}
	if err := r.ValidateInput("search", json.RawMessage(`{"query":"model comparison"}`)); err != nil {
		t.Errorf("valid input rejected: %v", err)
	}
	bad := map[string]string{
		"wrong type":    `{"query": 5}`,
		"missing field": `{}`,
		"extra field":   `{"query":"x","url":"file:///etc/passwd"}`,
		"not an object": `"query"`,
		"not JSON":      `{query`,
		"empty":         ``,
	}
	for name, input := range bad {
		if err := r.ValidateInput("search", json.RawMessage(input)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := r.ValidateInput("unknown", json.RawMessage(`{}`)); err == nil {
		t.Error("input for an unregistered tool was accepted")
	}
}
