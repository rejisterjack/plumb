package httpheader

import (
	"net/http"
	"testing"
)

func TestCloneForwardStripsHopHeaders(t *testing.T) {
	h := http.Header{
		"Stripe-Signature":  []string{"t=1,v1=abc"},
		"Content-Type":      []string{"application/json"},
		"Content-Length":    []string{"12"},
		"Connection":        []string{"close, X-Foo"},
		"X-Foo":             []string{"drop-me"},
		"Host":              []string{"relay.example"},
		"Transfer-Encoding": []string{"chunked"},
	}
	got := CloneForward(h)
	if got.Get("Stripe-Signature") != "t=1,v1=abc" {
		t.Fatalf("signature = %q", got.Get("Stripe-Signature"))
	}
	if got.Get("Content-Type") != "application/json" {
		t.Fatalf("content-type = %q", got.Get("Content-Type"))
	}
	for _, k := range []string{"Content-Length", "Connection", "X-Foo", "Host", "Transfer-Encoding"} {
		if got.Get(k) != "" {
			t.Fatalf("kept hop header %s = %q", k, got.Get(k))
		}
	}
}
