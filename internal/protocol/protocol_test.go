package protocol

import (
	"bytes"
	"net/http"
	"testing"
	"time"
)

func TestBodyRoundTrip(t *testing.T) {
	raw := []byte("{\"id\":1,\"sig\":\"a+b/c==\"}\n")
	enc := EncodeBody(raw)
	got, err := DecodeBody(enc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("body = %q, want %q", got, raw)
	}
	empty, err := DecodeBody("")
	if err != nil {
		t.Fatal(err)
	}
	if empty != nil {
		t.Fatalf("empty decode = %#v", empty)
	}
	if EncodeBody(nil) != "" {
		t.Fatal("empty encode should be blank")
	}
}

func TestMessageRoundTrip(t *testing.T) {
	msg := Message{
		Type:        TypeRequest,
		ID:          "req-1",
		Method:      http.MethodPost,
		URL:         "/webhooks/stripe?x=1",
		Host:        "relay.example",
		OriginalURL: "https://relay.example/h/abc/webhooks/stripe?x=1",
		Headers: http.Header{
			"Stripe-Signature": []string{"t=1,v1=abc"},
			"Content-Type":     []string{"application/json"},
		},
		Body:      EncodeBody([]byte(`{"ok":true}`)),
		TimeoutMs: 15000,
	}
	b, err := Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != TypeRequest || got.ID != "req-1" || got.URL != msg.URL {
		t.Fatalf("message = %+v", got)
	}
	if got.Headers.Get("Stripe-Signature") != "t=1,v1=abc" {
		t.Fatalf("signature header = %q", got.Headers.Get("Stripe-Signature"))
	}
	body, err := DecodeBody(got.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("body = %s", body)
	}
}

func TestReplayBatchRoundTrip(t *testing.T) {
	when := time.Date(2026, 10, 29, 1, 2, 3, 0, time.UTC)
	msg := Message{
		Type: TypeReplayBatch,
		Items: []Item{{
			ID:      "e1",
			Method:  http.MethodPost,
			URL:     "/hook",
			Headers: http.Header{"X-Hub-Signature-256": []string{"sha256=ab"}},
			Body:    EncodeBody([]byte("hi")),
			Time:    when,
		}},
	}
	b, err := Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("items = %d", len(got.Items))
	}
	if got.Items[0].Headers.Get("X-Hub-Signature-256") != "sha256=ab" {
		t.Fatalf("header = %q", got.Items[0].Headers.Get("X-Hub-Signature-256"))
	}
	if !got.Items[0].Time.Equal(when) {
		t.Fatalf("time = %s", got.Items[0].Time)
	}
}

func TestUnmarshalRejectsGarbage(t *testing.T) {
	if _, err := Unmarshal([]byte("{")); err == nil {
		t.Fatal("expected error")
	}
	if _, err := DecodeBody("not base64!!!"); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestValidSlug(t *testing.T) {
	ok := []string{"a", "alice", "dev_1", "A-B", "0123456789abcdef0123456789abcdef"}
	bad := []string{"", "has space", "a/b", "../x", "slug.sock", "hello!", string(make([]byte, 65))}
	for _, s := range ok {
		if !ValidSlug(s) {
			t.Errorf("ValidSlug(%q) = false", s)
		}
	}
	for _, s := range bad {
		if ValidSlug(s) {
			t.Errorf("ValidSlug(%q) = true", s)
		}
	}
}
