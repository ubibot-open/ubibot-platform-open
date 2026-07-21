package auth

import (
	"crypto/rand"
	"encoding/hex"
)

// NewOpaqueToken returns a random 32-byte value hex-encoded (64 chars),
// suitable as a bearer token for either a device session or an admin
// session — the two use the same shape (see store.IssueDeviceToken /
// store.IssueAdminSession) so there's one random-token routine to trust.
func NewOpaqueToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// NewDeviceSecret returns a random 16-byte value hex-encoded (32 chars),
// used when an operator provisions a device without supplying their own
// secret (see internal/api admin CreateDevice handler).
func NewDeviceSecret() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
