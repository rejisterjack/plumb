package relay

import "testing"

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
