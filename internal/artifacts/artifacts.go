package artifacts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/RoundtableCI/MerlinCI/internal/blob"
	"github.com/RoundtableCI/MerlinCI/pkg/tool"
	"github.com/RoundtableCI/MerlinCI/schemas"
)

// Artifact is a validated output with its provenance. It matches
// schemas/artifact.schema.json.
type Artifact struct {
	ID           string       `json:"id"`
	Kind         string       `json:"kind"`
	TaskID       string       `json:"taskId"`
	Role         string       `json:"role"`
	CampaignID   string       `json:"campaignId,omitempty"`
	Title        string       `json:"title"`
	Content      Content      `json:"content"`
	Sources      []Source     `json:"sources"`
	DerivedFrom  []string     `json:"derivedFrom,omitempty"`
	Evaluations  []Evaluation `json:"evaluations"`
	Cost         Cost         `json:"cost"`
	ConfigDigest string       `json:"configDigest"`
	CreatedAt    string       `json:"createdAt"`
}

// Content addresses the payload in object storage.
type Content struct {
	MediaType string `json:"mediaType"`
	BlobKey   string `json:"blobKey"`
	SHA256    string `json:"sha256"`
	SizeBytes int    `json:"sizeBytes"`
}

// Source is a piece of evidence the artifact traces to.
type Source struct {
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	RetrievedAt string `json:"retrievedAt"`
	SHA256      string `json:"sha256,omitempty"`
	Excerpt     string `json:"excerpt,omitempty"`
}

// Evaluation is one evaluator's verdict on the artifact.
type Evaluation struct {
	Evaluator string `json:"evaluator"`
	Verdict   string `json:"verdict"`
	Feedback  string `json:"feedback,omitempty"`
}

// Cost is what producing the artifact cost.
type Cost struct {
	Currency     string `json:"currency"`
	Amount       string `json:"amount"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
}

// Index stores artifact records. PostgreSQL implements it in production.
type Index interface {
	Insert(ctx context.Context, a Artifact) error
	Get(ctx context.Context, id string) (Artifact, bool, error)
}

// Draft is an output ready to be stored.
type Draft struct {
	ID           string
	Kind         string
	TaskID       string
	Role         string
	Title        string
	MediaType    string
	Body         []byte
	Sources      []tool.Source
	DerivedFrom  []string
	Evaluations  []Evaluation
	Cost         Cost
	ConfigDigest string
	CreatedAt    time.Time
}

// Store writes artifacts: the payload to object storage, the record to the
// index. A record that does not match the artifact schema is never stored.
type Store struct {
	Blobs blob.Store
	Index Index
}

// Save validates and stores a draft, and returns the stored record.
func (s *Store) Save(ctx context.Context, d Draft) (Artifact, error) {
	sum := sha256.Sum256(d.Body)
	a := Artifact{
		ID:     d.ID,
		Kind:   d.Kind,
		TaskID: d.TaskID,
		Role:   d.Role,
		Title:  d.Title,
		Content: Content{
			MediaType: d.MediaType,
			BlobKey:   "artifacts/" + d.ID,
			SHA256:    hex.EncodeToString(sum[:]),
			SizeBytes: len(d.Body),
		},
		Sources:      make([]Source, 0, len(d.Sources)),
		DerivedFrom:  d.DerivedFrom,
		Evaluations:  d.Evaluations,
		Cost:         d.Cost,
		ConfigDigest: d.ConfigDigest,
		CreatedAt:    d.CreatedAt.UTC().Format(time.RFC3339),
	}
	if a.Evaluations == nil {
		a.Evaluations = []Evaluation{}
	}
	for _, src := range d.Sources {
		a.Sources = append(a.Sources, Source{
			URL:         src.URL,
			Title:       src.Title,
			RetrievedAt: src.RetrievedAt.UTC().Format(time.RFC3339),
			SHA256:      src.SHA256,
			Excerpt:     src.Excerpt,
		})
	}
	if err := schemas.ValidateValue("Artifact", a); err != nil {
		return Artifact{}, fmt.Errorf("artifact does not match its schema: %w", err)
	}
	if err := s.Blobs.Put(ctx, a.Content.BlobKey, d.Body); err != nil {
		return Artifact{}, fmt.Errorf("store artifact payload: %w", err)
	}
	if err := s.Index.Insert(ctx, a); err != nil {
		return Artifact{}, fmt.Errorf("store artifact record: %w", err)
	}
	return a, nil
}

// MemoryIndex is an Index kept in memory, for tests and local runs.
type MemoryIndex struct {
	mu   sync.Mutex
	byID map[string]Artifact
}

// NewMemoryIndex returns an empty in-memory index.
func NewMemoryIndex() *MemoryIndex { return &MemoryIndex{byID: map[string]Artifact{}} }

func (m *MemoryIndex) Insert(_ context.Context, a Artifact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, dup := m.byID[a.ID]; dup {
		return fmt.Errorf("artifact %q already exists", a.ID)
	}
	m.byID[a.ID] = a
	return nil
}

func (m *MemoryIndex) Get(_ context.Context, id string) (Artifact, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.byID[id]
	return a, ok, nil
}
