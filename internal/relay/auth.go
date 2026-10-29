package relay

import (
	"crypto/sha256"
	"crypto/subtle"
)

// tokenMatch compares tokens without leaking the length or contents through
// an early return. Both sides are hashed so different lengths still take the
// same time.
func tokenMatch(got, want string) bool {
	g := sha256.Sum256([]byte(got))
	w := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(g[:], w[:]) == 1
}
