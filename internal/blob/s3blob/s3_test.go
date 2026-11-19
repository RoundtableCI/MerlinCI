package s3blob

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/RoundtableCI/MerlinCI/internal/blob"
)

var ctx = context.Background()

// contract is what every Store must do.
func contract(t *testing.T, s blob.Store) {
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

// TestS3 needs an S3-compatible service. Set MERLIN_TEST_S3_ENDPOINT to run
// it, for example against deploy/docker-compose.yml:
//
//	MERLIN_TEST_S3_ENDPOINT=http://localhost:8333
func TestS3(t *testing.T) {
	endpoint := os.Getenv("MERLIN_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("MERLIN_TEST_S3_ENDPOINT is not set")
	}
	cfg := Config{
		Endpoint:        endpoint,
		Region:          "us-east-1",
		Bucket:          fmt.Sprintf("merlin-test-%d", time.Now().UnixNano()),
		AccessKeyID:     os.Getenv("MERLIN_TEST_S3_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("MERLIN_TEST_S3_SECRET_ACCESS_KEY"),
	}
	s, err := New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	contract(t, s)

	// Connecting again finds the bucket and the content already there.
	again, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("second New: %v", err)
	}
	if got, err := again.Get(ctx, "artifacts/art_1"); err != nil || !strings.HasPrefix(string(got), "# An opportunity") {
		t.Fatalf("content did not persist across connections: %q, %v", got, err)
	}
}

func TestS3RejectsBadConfig(t *testing.T) {
	if _, err := New(ctx, Config{Endpoint: "not a url", Bucket: "b"}); err == nil {
		t.Error("an endpoint that is not a URL was accepted")
	}
	if _, err := New(ctx, Config{Endpoint: "http://localhost:8333"}); err == nil {
		t.Error("a missing bucket name was accepted")
	}
}
