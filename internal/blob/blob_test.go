package blob

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

var ctx = context.Background()

// contract is what every Store must do.
func contract(t *testing.T, s Store) {
	t.Helper()
	body := []byte("# An opportunity\nPeople ask how to compare models.\n")
	if err := s.Put(ctx, "artifacts/art_1", body); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := s.Get(ctx, "artifacts/art_1")
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("Get = %q, %v; want what was put", got, err)
	}
	if err := s.Put(ctx, "artifacts/empty", nil); err != nil {
		t.Fatalf("Put of empty content: %v", err)
	}
	if got, err := s.Get(ctx, "artifacts/empty"); err != nil || len(got) != 0 {
		t.Fatalf("Get of empty content = %q, %v", got, err)
	}
	if _, err := s.Get(ctx, "artifacts/missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Get of a missing key: err = %v, want not found", err)
	}
	// A larger payload, such as a long article.
	large := bytes.Repeat([]byte("0123456789abcdef"), 64<<10) // 1 MiB
	if err := s.Put(ctx, "artifacts/large", large); err != nil {
		t.Fatalf("Put of 1 MiB: %v", err)
	}
	if got, err := s.Get(ctx, "artifacts/large"); err != nil || !bytes.Equal(got, large) {
		t.Fatalf("1 MiB did not survive the round trip: %d bytes, %v", len(got), err)
	}
}

func TestMemory(t *testing.T) {
	m := NewMemory()
	contract(t, m)

	// The store keeps its own copy: changing the caller's slice changes nothing.
	data := []byte("original")
	_ = m.Put(ctx, "k", data)
	data[0] = 'X'
	if got, _ := m.Get(ctx, "k"); string(got) != "original" {
		t.Errorf("stored content changed with the caller's slice: %q", got)
	}
}
