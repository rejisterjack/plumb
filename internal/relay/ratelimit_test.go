package relay

import (
	"strconv"
	"testing"
)

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

func TestLimiterCapsVisitors(t *testing.T) {
	l := newLimiter(0, 1)
	for i := 0; i < maxVisitors+100; i++ {
		l.Allow(strconv.Itoa(i))
	}
	l.mu.Lock()
	n := len(l.visitors)
	l.mu.Unlock()
	if n > maxVisitors+1 {
		t.Fatalf("visitors = %d", n)
	}
}
