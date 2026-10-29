package tunnel

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestLogExchangeTruncatesBody(t *testing.T) {
	body := bytes.Repeat([]byte("a"), maxDisplayBody+10)
	var buf bytes.Buffer
	LogExchange(&buf, false, http.MethodPost, "/hook", 200, 1500*time.Microsecond, http.Header{
		"Stripe-Signature": []string{"t=1"},
	}, body, "")
	out := buf.String()
	if !strings.Contains(out, "POST /hook -> 200") {
		t.Fatalf("missing summary: %s", out)
	}
	if !strings.Contains(out, "Stripe-Signature: t=1") {
		t.Fatalf("missing header: %s", out)
	}
	if !strings.Contains(out, "truncated") {
		t.Fatal("expected truncation marker")
	}
	if strings.Contains(out, strings.Repeat("a", maxDisplayBody+10)) {
		t.Fatal("logged the full body")
	}
}

func TestLogExchangeReplayAndBinary(t *testing.T) {
	var buf bytes.Buffer
	LogExchange(&buf, true, http.MethodPost, "/hook", 204, 0, nil, []byte{0xff, 0xfe}, "")
	out := buf.String()
	if !strings.HasPrefix(out, "replay POST /hook -> 204") {
		t.Fatalf("summary = %q", out)
	}
	if !strings.Contains(out, "binary 2 bytes") {
		t.Fatalf("body = %s", out)
	}
}
