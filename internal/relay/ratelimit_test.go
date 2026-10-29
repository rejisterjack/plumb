package relay

import "testing"

func TestLimiterBurst(t *testing.T) {
	l := newLimiter(0, 1)
	if !l.Allow("1.2.3.4") {
		t.Fatal("first request should pass")
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("second request should be limited")
	}
	if !l.Allow("5.6.7.8") {
		t.Fatal("other address should have its own bucket")
	}
}
