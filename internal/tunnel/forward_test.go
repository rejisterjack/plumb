package tunnel

import "testing"

func TestSafePath(t *testing.T) {
	ok, err := SafePath("/webhooks/stripe?x=1")
	if err != nil || ok != "/webhooks/stripe?x=1" {
		t.Fatalf("got %q %v", ok, err)
	}
	if _, err := SafePath(""); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"http://evil", "//evil", "/webhooks/../secret", `/\windows`, "/%2e%2e/secret"} {
		if _, err := SafePath(bad); err == nil {
			t.Errorf("SafePath(%q) accepted", bad)
		}
	}
}

func TestPublicAndTunnelURL(t *testing.T) {
	pub, err := PublicURL("https://relay.example/ignored", "abc")
	if err != nil {
		t.Fatal(err)
	}
	if pub != "https://relay.example/h/abc/" {
		t.Fatalf("public = %s", pub)
	}
	ws, err := TunnelURL("https://relay.example")
	if err != nil {
		t.Fatal(err)
	}
	if ws != "wss://relay.example/tunnel" {
		t.Fatalf("tunnel = %s", ws)
	}
	if _, err := PublicURL("ftp://relay.example", "abc"); err == nil {
		t.Fatal("expected scheme error")
	}
}
