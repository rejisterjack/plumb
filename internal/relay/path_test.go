package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSplitHookPath(t *testing.T) {
	tests := []struct {
		in   string
		slug string
		rest string
		ok   bool
	}{
		{in: "/h/abc123/webhooks/stripe", slug: "abc123", rest: "/webhooks/stripe", ok: true},
		{in: "/h/abc123/webhooks/stripe?x=1", slug: "abc123", rest: "/webhooks/stripe?x=1", ok: true},
		{in: "/h/abc123", slug: "abc123", rest: "/", ok: true},
		{in: "/h/abc123/", slug: "abc123", rest: "/", ok: true},
		{in: "/h/dev_1/a/b", slug: "dev_1", rest: "/a/b", ok: true},
		{in: "/healthz", ok: false},
		{in: "/tunnel", ok: false},
		{in: "/h/", ok: false},
		{in: "/h/has space/x", ok: false},
		{in: "/h/../etc", ok: false},
		{in: "/", ok: false},
	}
	for _, tt := range tests {
		slug, rest, ok := SplitHookPath(tt.in)
		if ok != tt.ok || slug != tt.slug || rest != tt.rest {
			t.Errorf("SplitHookPath(%q) = (%q, %q, %v), want (%q, %q, %v)", tt.in, slug, rest, ok, tt.slug, tt.rest, tt.ok)
		}
	}
}

func TestUncleanRequestPath(t *testing.T) {
	srv := New(Config{Token: "secret"}, nil)
	t.Cleanup(srv.Close)
	h := srv.Handler()
	for _, p := range []string{"/h/dev/../../secret", "/h/dev/./hook", "/h/dev//hook"} {
		req := httptest.NewRequest(http.MethodPost, "http://relay.example/h/dev/hook", nil)
		req.URL.Path = p
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status %d", p, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "http://relay.example/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "http://relay.example/h/dev/", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("trailing slash status %d", rec.Code)
	}
}
