package relay

import (
	"sync"
	"time"
)

// limiter is a per-key token bucket. It exists to blunt accidental floods,
// not to be a precise quota system.
type limiter struct {
	mu       sync.Mutex
	rate     float64
	burst    float64
	visitors map[string]*visitor
}

type visitor struct {
	tokens  float64
	updated time.Time
}

func newLimiter(rate, burst float64) *limiter {
	if burst < 1 {
		burst = 1
	}
	if rate < 0 {
		rate = 0
	}
	return &limiter{
		rate:     rate,
		burst:    burst,
		visitors: make(map[string]*visitor),
	}
}

func (l *limiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.visitors) > 10000 {
		for k, v := range l.visitors {
			if now.Sub(v.updated) > 10*time.Minute {
				delete(l.visitors, k)
			}
		}
	}
	v := l.visitors[key]
	if v == nil {
		v = &visitor{tokens: l.burst, updated: now}
		l.visitors[key] = v
	} else {
		elapsed := now.Sub(v.updated).Seconds()
		v.updated = now
		v.tokens += elapsed * l.rate
		if v.tokens > l.burst {
			v.tokens = l.burst
		}
	}
	if v.tokens < 1 {
		return false
	}
	v.tokens--
	return true
}
