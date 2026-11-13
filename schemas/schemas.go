// Package schemas embeds the JSON Schemas of the open agent configuration
// format and the core records (deck slides 16 and 19), and validates documents
// against them.
package schemas

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed *.schema.json
var files embed.FS

// APIVersion is the version every configuration document must declare.
const APIVersion = "merlinci/v1alpha1"

// refs maps a document or record kind to the schema that describes it.
var refs = map[string]string{
	"Profile":     "profile.schema.json#/$defs/Profile",
	"Audiences":   "profile.schema.json#/$defs/Audiences",
	"RuntimeRole": "role.schema.json#/$defs/RuntimeRole",
	"Role":        "role.schema.json#/$defs/Role",
	"Policy":      "policy.schema.json",
	"Budget":      "budget.schema.json",
	"Workflow":    "workflow.schema.json",
	"Models":      "models.schema.json",
	"Task":        "task.schema.json",
	"Artifact":    "artifact.schema.json",
}

const base = "mem:///"

var compiled = sync.OnceValues(func() (map[string]*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		raw, err := files.ReadFile(e.Name())
		if err != nil {
			return nil, err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if err := c.AddResource(base+e.Name(), doc); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
	}
	out := make(map[string]*jsonschema.Schema, len(refs))
	for kind, ref := range refs {
		s, err := c.Compile(base + ref)
		if err != nil {
			return nil, fmt.Errorf("compile schema for %s: %w", kind, err)
		}
		out[kind] = s
	}
	return out, nil
})

// Known reports whether kind has a schema.
func Known(kind string) bool {
	_, ok := refs[kind]
	return ok
}

// Validate checks a JSON document against the schema for kind.
func Validate(kind string, document []byte) error {
	all, err := compiled()
	if err != nil {
		return err
	}
	s, ok := all[kind]
	if !ok {
		return fmt.Errorf("unknown kind %q", kind)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(document))
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return s.Validate(inst)
}

// ValidateValue marshals v to JSON and validates it against the schema for kind.
func ValidateValue(kind string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return Validate(kind, raw)
}
