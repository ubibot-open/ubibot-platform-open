package auth

import (
	"crypto/rand"
	"encoding/hex"
)

// NewOpaqueToken returns a random 32-byte value hex-encoded (64 chars),
// suitable as a bearer token for an admin session (see
// store.IssueAdminSession) — the device-facing protocol has no session
// token of its own anymore (docs §2/§3/§4), so this is the only caller.
func NewOpaqueToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
