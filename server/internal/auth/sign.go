// Package auth implements the HMAC-SHA256 device signature scheme, the
// single-use nonce used for the clockless first-activation flow, and
// session token issuance/validation described in
// docs/UbiBot开放平台硬件通信协议.docx.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// Sign computes HEX(HMAC-SHA256(secret, concatenated parts)), matching the
// device-side signing rule for both /auth/time (pid+sn) and /auth/activate
// (pid+sn+ts+n).
func Sign(secret string, parts ...string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	for _, p := range parts {
		mac.Write([]byte(p))
	}
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify reports whether sign matches HMAC-SHA256(secret, parts...) using a
// constant-time comparison.
func Verify(secret string, sign string, parts ...string) bool {
	expected := Sign(secret, parts...)
	return hmac.Equal([]byte(expected), []byte(sign))
}

// FormatTs renders a Unix-second timestamp the same way the device embeds it
// in the signed string (decimal, no separators).
func FormatTs(ts int64) string {
	return strconv.FormatInt(ts, 10)
}
