package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/RoundtableCI/MerlinCI/pkg/tool"
)

// Registry holds the tools MerlinCI can run. A tool that is not registered
// cannot be called, whatever the model proposes.
type Registry struct {
	tools   map[string]tool.Tool
	schemas map[string]*jsonschema.Schema
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{tools: map[string]tool.Tool{}, schemas: map[string]*jsonschema.Schema{}}
}

// Register adds a tool. It fails when the tool has no name or action, when
// the name is taken, or when its input schema does not compile.
func (r *Registry) Register(t tool.Tool) error {
	spec := t.Spec()
	if spec.Name == "" || spec.Action == "" {
		return fmt.Errorf("tool needs a name and an action")
	}
	if _, dup := r.tools[spec.Name]; dup {
		return fmt.Errorf("tool %q is already registered", spec.Name)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(spec.InputSchema))
	if err != nil {
		return fmt.Errorf("tool %q: input schema: %w", spec.Name, err)
	}
	c := jsonschema.NewCompiler()
	url := "mem:///tools/" + spec.Name
	if err := c.AddResource(url, doc); err != nil {
		return fmt.Errorf("tool %q: input schema: %w", spec.Name, err)
	}
	schema, err := c.Compile(url)
	if err != nil {
		return fmt.Errorf("tool %q: input schema: %w", spec.Name, err)
	}
	r.tools[spec.Name] = t
	r.schemas[spec.Name] = schema
	return nil
}

// Get returns a registered tool.
func (r *Registry) Get(name string) (tool.Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Names returns the registered tool names in order.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ValidateInput checks proposed input against the tool's schema.
func (r *Registry) ValidateInput(name string, input json.RawMessage) error {
	schema, ok := r.schemas[name]
	if !ok {
		return fmt.Errorf("tool %q is not registered", name)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(input))
	if err != nil {
		return fmt.Errorf("input is not valid JSON: %w", err)
	}
	return schema.Validate(inst)
}
