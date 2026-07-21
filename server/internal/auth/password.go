package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// HashPassword and VerifyPassword implement PBKDF2-HMAC-SHA256 by hand
// instead of reaching for golang.org/x/crypto/bcrypt — this environment's
// module proxy can't reach golang.org, and hand-rolling PBKDF2 (it's just
// iterated HMAC) avoids that dependency entirely. Device authentication
// never goes through this file — devices sign with HMAC and a factory
// secret (see sign.go), a different threat model than an admin's chosen
// password.

const (
	pbkdf2Iterations = 100_000
	pbkdf2KeyLen      = 32
	pbkdf2SaltLen     = 16
)

// pbkdf2 derives a keyLen-byte key from password+salt using iter rounds of
// HMAC-SHA256, per RFC 8018 with a single-block output (32 bytes fits in
// one SHA-256 block, so no block-counter loop is needed).
func pbkdf2(password, salt []byte, iter, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	prf.Write(salt)
	prf.Write([]byte{0, 0, 0, 1}) // block index 1, big-endian uint32

	u := prf.Sum(nil)
	result := make([]byte, len(u))
	copy(result, u)

	for i := 1; i < iter; i++ {
		prf.Reset()
		prf.Write(u)
		u = prf.Sum(nil)
		for j := range result {
			result[j] ^= u[j]
		}
	}
	return result[:keyLen]
}

// HashPassword returns a self-describing hash string:
// "pbkdf2-sha256$<iterations>$<salt-hex>$<hash-hex>" — the iteration count
// and salt travel with the hash so VerifyPassword doesn't need them passed
// separately, and so the cost can be raised later without invalidating
// hashes written under the old value.
func HashPassword(plain string) (string, error) {
	salt := make([]byte, pbkdf2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	derived := pbkdf2([]byte(plain), salt, pbkdf2Iterations, pbkdf2KeyLen)
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", pbkdf2Iterations, hex.EncodeToString(salt), hex.EncodeToString(derived)), nil
}

// VerifyPassword reports whether plain matches hash, using a
// constant-time comparison on the derived key.
func VerifyPassword(hash, plain string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return false
	}
	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}

	got := pbkdf2([]byte(plain), salt, iter, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1
}
