package openrouter

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RoundtableCI/MerlinCI/pkg/provider"
)

func TestDollarsToMicros(t *testing.T) {
	good := map[string]int64{
		"0":          0,
		"1":          1_000_000,
		"0.00042":    420,
		"4.2e-05":    42,
		"4.2E-5":     42,
		"0.0000001":  1, // a tenth of a millionth still costs something: rounded up
		"0.0000014":  2,
		"12.3456789": 12_345_679,
	}
	for in, want := range good {
		if got, ok := dollarsToMicros(in); !ok || got != want {
			t.Errorf("dollarsToMicros(%q) = %d, %v; want %d", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "abc", "-0.01", "1e400"} {
		if got, ok := dollarsToMicros(in); ok {
			t.Errorf("dollarsToMicros(%q) = %d; want it refused", in, got)
		}
	}
}

func serve(t *testing.T, status int, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, key, srv.Client())
}

func TestTruncatedReplyIsReported(t *testing.T) {
	c := serve(t, 200, `{"choices": [{"message": {"role": "assistant", "content": "half an artic"}, "finish_reason": "length"}], "usage": {"prompt_tokens": 5, "completion_tokens": 4096}}`)
	resp, err := c.Complete(context.Background(), provider.Request{Model: "m"})
	if err != nil || !resp.Truncated {
		t.Fatalf("truncated = %v, %v; want true", resp.Truncated, err)
	}
	c = serve(t, 200, `{"choices": [{"message": {"role": "assistant", "content": "done"}, "finish_reason": "stop"}], "usage": {"prompt_tokens": 5, "completion_tokens": 1}}`)
	if resp, _ := c.Complete(context.Background(), provider.Request{Model: "m"}); resp.Truncated {
		t.Error("a complete reply was reported as truncated")
	}
}

// Which failures certainly cost nothing, and which might have been billed.
func TestWhichFailuresAreNotBilled(t *testing.T) {
	req := provider.Request{Model: "m"}

	// The provider refused the request.
	for _, status := range []int{400, 402, 429, 500, 503} {
		_, err := serve(t, status, `{"error": {"message": "no"}}`).Complete(context.Background(), req)
		if !errors.Is(err, provider.ErrNotBilled) {
			t.Errorf("status %d: err = %v, want ErrNotBilled", status, err)
		}
	}
	// A rejected key is a credential problem, and also not billed.
	_, err := serve(t, 401, `{"error": {"message": "bad key"}}`).Complete(context.Background(), req)
	if !errors.Is(err, provider.ErrCredential) {
		t.Errorf("status 401: err = %v, want ErrCredential", err)
	}

	// The connection was never made.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()
	if _, err := New(url, key, nil).Complete(context.Background(), req); !errors.Is(err, provider.ErrNotBilled) {
		t.Errorf("unreachable server: err = %v, want ErrNotBilled", err)
	}

	// The provider answered 200, so it may have billed, but the answer is unusable.
	for name, body := range map[string]string{
		"unreadable body": `{"choices": [`,
		"no choices":      `{"choices": [], "usage": {"prompt_tokens": 5}}`,
		"error in a 200":  `{"error": {"message": "upstream failed"}}`,
	} {
		_, err := serve(t, 200, body).Complete(context.Background(), req)
		if err == nil || errors.Is(err, provider.ErrNotBilled) {
			t.Errorf("%s: err = %v; want an error that is NOT marked as not billed", name, err)
		}
	}
}

// A cost the provider wrote in a form that cannot be read is treated as not
// reported, so the gateway prices the call from its token count instead.
func TestUnreadableCostIsLeftUnreported(t *testing.T) {
	c := serve(t, 200, `{"choices": [{"message": {"role": "assistant", "content": "hi"}}], "usage": {"prompt_tokens": 5, "completion_tokens": 1, "cost": -1}}`)
	resp, err := c.Complete(context.Background(), provider.Request{Model: "m"})
	if err != nil || resp.Usage.Currency != "" || resp.Usage.InputTokens != 5 {
		t.Fatalf("usage = %+v, %v; want tokens and no cost", resp.Usage, err)
	}
}
