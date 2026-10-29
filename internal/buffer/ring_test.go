package buffer

import (
	"bytes"
	"net/http"
	"strconv"
	"sync"
	"testing"
)

func TestRingOverflowKeepsNewest(t *testing.T) {
	r := New(2)
	r.Add(Entry{ID: "1", Body: []byte("a")})
	r.Add(Entry{ID: "2", Body: []byte("b")})
	r.Add(Entry{ID: "3", Body: []byte("c")})

	if r.Len() != 2 {
		t.Fatalf("len = %d", r.Len())
	}
	if r.Cap() != 2 {
		t.Fatalf("cap = %d", r.Cap())
	}
	got := r.Last(5)
	if len(got) != 2 || got[0].ID != "2" || got[1].ID != "3" {
		t.Fatalf("last = %+v", ids(got))
	}
	one := r.Last(1)
	if len(one) != 1 || one[0].ID != "3" {
		t.Fatalf("last 1 = %+v", ids(one))
	}
	if r.Last(0) != nil {
		t.Fatal("last 0 should be nil")
	}
}

func TestRingCopiesBodyAndHeaders(t *testing.T) {
	body := []byte("original")
	h := http.Header{"Stripe-Signature": []string{"v1"}}
	r := New(1)
	r.Add(Entry{ID: "1", Headers: h, Body: body})
	body[0] = 'X'
	h.Set("Stripe-Signature", "changed")

	got := r.Last(1)[0]
	if !bytes.Equal(got.Body, []byte("original")) {
		t.Fatalf("body mutated: %q", got.Body)
	}
	if got.Headers.Get("Stripe-Signature") != "v1" {
		t.Fatalf("header mutated: %q", got.Headers.Get("Stripe-Signature"))
	}
}

func TestRingNonPositiveCap(t *testing.T) {
	r := New(0)
	r.Add(Entry{ID: "1"})
	r.Add(Entry{ID: "2"})
	got := r.Last(10)
	if len(got) != 1 || got[0].ID != "2" {
		t.Fatalf("got %+v", ids(got))
	}
}

func TestRingConcurrentAdds(t *testing.T) {
	r := New(50)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.Add(Entry{ID: strconv.Itoa(n), Body: []byte("x")})
		}(i)
	}
	wg.Wait()
	if r.Len() != 50 {
		t.Fatalf("len = %d", r.Len())
	}
}

func ids(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.ID
	}
	return out
}
