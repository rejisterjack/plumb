package relay

import "testing"

func TestTokenMatch(t *testing.T) {
	if !tokenMatch("secret", "secret") {
		t.Fatal("expected match")
	}
	if tokenMatch("secret", "secret2") || tokenMatch("secret", "") || tokenMatch("", "x") {
		t.Fatal("expected mismatch")
	}
	// Equal strings match, including empty. Refusing an empty server token is
	// Server.authorized's job, so a public relay cannot be claimed without --dev.
	if !tokenMatch("", "") {
		t.Fatal("equal strings should match")
	}
}
