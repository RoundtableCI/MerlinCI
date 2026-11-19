package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/RoundtableCI/MerlinCI/schemas"
)

// Severity of a Problem.
type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
)

// Problem is one finding from loading or validating configuration.
type Problem struct {
	Severity Severity
	File     string // empty when the problem concerns the bundle as a whole
	Subject  string // "Kind/name", when known
	Message  string
}

func (p Problem) String() string {
	var b strings.Builder
	if p.File != "" {
		b.WriteString(p.File + ": ")
	}
	b.WriteString(string(p.Severity) + ": ")
	if p.Subject != "" {
		b.WriteString(p.Subject + ": ")
	}
	b.WriteString(p.Message)
	return b.String()
}

// Problems is a list of findings.
type Problems []Problem

// HasErrors reports whether any problem is an error.
func (ps Problems) HasErrors() bool {
	for _, p := range ps {
		if p.Severity == Error {
			return true
		}
	}
	return false
}

// Count returns the number of problems with the given severity.
func (ps Problems) Count(s Severity) int {
	n := 0
	for _, p := range ps {
		if p.Severity == s {
			n++
		}
	}
	return n
}

// Bundle is a loaded set of configuration documents.
type Bundle struct {
	Profile      *Profile
	Audiences    *Audiences
	Policy       *Policy
	Budget       *Budget
	Models       *Models
	RuntimeRoles map[string]RuntimeRole
	Roles        map[string]Role
	Workflows    map[string]Workflow

	// Digest identifies this exact configuration. Runs and artifacts record
	// it so that what MerlinCI did can be traced to the configuration it
	// did it under.
	Digest string

	files     map[string]string // "Kind/name" -> file it was loaded from
	canonical map[string][]byte // "Kind/name" -> canonical JSON
}

// Documents returns the number of documents in the bundle.
func (b *Bundle) Documents() int { return len(b.files) }

// Load reads every .yaml and .yml file under the given paths, validates each
// document against its schema, and checks the documents against each other.
// The bundle is usable only when the returned problems contain no errors.
func Load(paths ...string) (*Bundle, Problems) {
	b := &Bundle{
		RuntimeRoles: map[string]RuntimeRole{},
		Roles:        map[string]Role{},
		Workflows:    map[string]Workflow{},
		files:        map[string]string{},
		canonical:    map[string][]byte{},
	}
	var ps Problems

	var files []string
	for _, root := range paths {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			ext := strings.ToLower(filepath.Ext(path))
			if !d.IsDir() && (ext == ".yaml" || ext == ".yml") {
				files = append(files, filepath.ToSlash(path))
			}
			return nil
		})
		if err != nil {
			ps = append(ps, Problem{Severity: Error, File: filepath.ToSlash(root), Message: err.Error()})
		}
	}
	sort.Strings(files)

	for _, file := range files {
		ps = append(ps, b.loadFile(file)...)
	}
	ps = append(ps, b.check()...)
	b.Digest = b.digest()
	return b, ps
}

func (b *Bundle) loadFile(file string) Problems {
	raw, err := os.ReadFile(file)
	if err != nil {
		return Problems{{Severity: Error, File: file, Message: err.Error()}}
	}
	var ps Problems
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	for n := 1; ; n++ {
		var doc any
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			ps = append(ps, Problem{Severity: Error, File: file, Message: "invalid YAML: " + err.Error()})
			break
		}
		if doc == nil {
			continue // empty document
		}
		if p := b.add(file, doc); p != nil {
			if n > 1 {
				p.Message = fmt.Sprintf("document %d: %s", n, p.Message)
			}
			ps = append(ps, *p)
		}
	}
	return ps
}

// add validates one decoded YAML document and stores it in the bundle.
func (b *Bundle) add(file string, doc any) *Problem {
	fail := func(subject, msg string) *Problem {
		return &Problem{Severity: Error, File: file, Subject: subject, Message: msg}
	}

	// encoding/json writes object keys in sorted order, so this is canonical.
	canonical, err := json.Marshal(doc)
	if err != nil {
		return fail("", "not representable as JSON: "+err.Error())
	}
	var head struct {
		Kind     string `json:"kind"`
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(canonical, &head); err != nil || head.Kind == "" {
		return fail("", "document must be a mapping with a kind")
	}
	if !isConfigKind(head.Kind) {
		return fail("", fmt.Sprintf("unknown kind %q", head.Kind))
	}
	subject := head.Kind + "/" + head.Metadata.Name

	if err := schemas.Validate(head.Kind, canonical); err != nil {
		return fail(subject, err.Error())
	}
	if prev, dup := b.files[subject]; dup {
		return fail(subject, "already defined in "+prev)
	}

	switch head.Kind {
	case KindProfile:
		err = single(&b.Profile, canonical, b.files, KindProfile)
	case KindAudiences:
		err = single(&b.Audiences, canonical, b.files, KindAudiences)
	case KindPolicy:
		err = single(&b.Policy, canonical, b.files, KindPolicy)
	case KindBudget:
		err = single(&b.Budget, canonical, b.files, KindBudget)
	case KindModels:
		err = single(&b.Models, canonical, b.files, KindModels)
	case KindRuntimeRole:
		err = keyed(b.RuntimeRoles, head.Metadata.Name, canonical)
	case KindRole:
		err = keyed(b.Roles, head.Metadata.Name, canonical)
	case KindWorkflow:
		err = keyed(b.Workflows, head.Metadata.Name, canonical)
	}
	if err != nil {
		return fail(subject, err.Error())
	}
	b.files[subject] = file
	b.canonical[subject] = canonical
	return nil
}

func isConfigKind(kind string) bool {
	switch kind {
	case KindProfile, KindAudiences, KindPolicy, KindBudget, KindModels, KindRuntimeRole, KindRole, KindWorkflow:
		return true
	}
	return false
}

// decode unmarshals strictly: a field the Go types do not know is an error,
// so the schemas and the types cannot drift apart silently.
func decode[T any](raw []byte) (T, error) {
	var v T
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	err := dec.Decode(&v)
	return v, err
}

// single stores a kind that a deployment has exactly one of.
func single[T any](dst **T, raw []byte, files map[string]string, kind string) error {
	if *dst != nil {
		for subject, file := range files {
			if strings.HasPrefix(subject, kind+"/") {
				return fmt.Errorf("a deployment has one %s; %s is already defined in %s", kind, subject, file)
			}
		}
	}
	v, err := decode[T](raw)
	if err != nil {
		return err
	}
	*dst = &v
	return nil
}

func keyed[T any](dst map[string]T, name string, raw []byte) error {
	v, err := decode[T](raw)
	if err != nil {
		return err
	}
	dst[name] = v
	return nil
}

func (b *Bundle) digest() string {
	subjects := make([]string, 0, len(b.canonical))
	for s := range b.canonical {
		subjects = append(subjects, s)
	}
	sort.Strings(subjects)
	h := sha256.New()
	for _, s := range subjects {
		fmt.Fprintf(h, "%s\n%d\n", s, len(b.canonical[s]))
		h.Write(b.canonical[s])
		h.Write([]byte{'\n'})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
