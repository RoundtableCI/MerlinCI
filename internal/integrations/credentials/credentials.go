package credentials

import (
	"fmt"
	"os"
)

// Names of the credentials MerlinCI knows about.
const (
	OpenRouter  = "OPENROUTER_API_KEY"
	BraveSearch = "BRAVE_SEARCH_API_KEY"
)

// Store is the one place credentials are read from. Connectors receive a
// getter for the single credential they need; nothing else in MerlinCI,
// and nothing a model can reach, has access to this store.
type Store struct {
	lookup func(name string) (string, bool)
}

// FromEnv returns a store that reads the process environment.
func FromEnv() *Store { return &Store{lookup: os.LookupEnv} }

// New returns a store backed by lookup. Tests use it.
func New(lookup func(name string) (string, bool)) *Store { return &Store{lookup: lookup} }

// Has reports whether a credential is set.
func (s *Store) Has(name string) bool {
	v, ok := s.lookup(name)
	return ok && v != ""
}

// Getter returns a function that reads one credential at the moment it is
// needed. The error it returns names the credential and never its value.
func (s *Store) Getter(name string) func() (string, error) {
	return func() (string, error) {
		v, ok := s.lookup(name)
		if !ok || v == "" {
			return "", fmt.Errorf("%s is not set", name)
		}
		return v, nil
	}
}
