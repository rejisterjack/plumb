package tunnel

import (
	"testing"
	"time"
)

func TestBackoffStaysInRange(t *testing.T) {
	min := 500 * time.Millisecond
	max := 30 * time.Second
	for attempt := 0; attempt < 12; attempt++ {
		d := backoff(attempt, min, max)
		if d > max {
			t.Fatalf("attempt %d: %s exceeds max", attempt, d)
		}
		if d < min/2 {
			t.Fatalf("attempt %d: %s below floor", attempt, d)
		}
	}
	if backoff(20, min, max) > max {
		t.Fatal("capped backoff exceeded max")
	}
}
