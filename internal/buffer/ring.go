// Package buffer is a fixed-size in-memory ring of recent webhook requests.
// Contents are lost when the relay process exits.
package buffer

import (
	"bytes"
	"net/http"
	"sync"
	"time"
)

// Entry is one accepted webhook, stored as the raw bytes the provider sent.
type Entry struct {
	ID          string
	Method      string
	URL         string
	Host        string
	OriginalURL string
	Headers     http.Header
	Body        []byte
	Time        time.Time
}

// Ring keeps the newest n entries. Add drops the oldest when full.
type Ring struct {
	mu    sync.Mutex
	items []Entry
	cap   int
}

// New returns a ring that retains cap entries. A non-positive cap retains one
// entry so a misconfigured relay still has a replay slot.
func New(cap int) *Ring {
	if cap < 1 {
		cap = 1
	}
	return &Ring{cap: cap}
}

// Cap returns the maximum number of retained entries.
func (r *Ring) Cap() int { return r.cap }

// Len returns how many entries are currently stored.
func (r *Ring) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.items)
}

// Add copies e into the ring, dropping the oldest entry when at capacity.
func (r *Ring) Add(e Entry) {
	e.Body = bytes.Clone(e.Body)
	if e.Headers != nil {
		e.Headers = e.Headers.Clone()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = append(r.items, e)
	if len(r.items) > r.cap {
		next := make([]Entry, r.cap)
		copy(next, r.items[len(r.items)-r.cap:])
		r.items = next
	}
}

// Last returns up to n entries, oldest first. n <= 0 returns nil.
func (r *Ring) Last(n int) []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n <= 0 || len(r.items) == 0 {
		return nil
	}
	if n > len(r.items) {
		n = len(r.items)
	}
	out := make([]Entry, n)
	copy(out, r.items[len(r.items)-n:])
	return out
}
