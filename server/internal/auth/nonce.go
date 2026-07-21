package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// NonceTTL is how long a nonce issued by /auth/time stays valid, per the doc.
const NonceTTL = 60 * time.Second

// TimeWindow is the tolerance used to validate ts when a device signs an
// activation request with its own clock instead of a nonce.
const TimeWindow = 5 * time.Minute

type nonceEntry struct {
	value     string
	expiresAt time.Time
}

// NonceStore issues and single-use-consumes nonces keyed by device serial
// number. It is the anti-replay mechanism for devices with no reliable
// clock: the nonce, not a timestamp window, proves the activation request
// is fresh.
type NonceStore struct {
	mu sync.Mutex
	m  map[string]nonceEntry
}

func NewNonceStore() *NonceStore {
	return &NonceStore{m: make(map[string]nonceEntry)}
}

// Issue creates a new nonce for sn, replacing any previously issued,
// unconsumed nonce for the same device.
func (s *NonceStore) Issue(sn string, now time.Time) (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	n := hex.EncodeToString(buf)

	s.mu.Lock()
	s.m[sn] = nonceEntry{value: n, expiresAt: now.Add(NonceTTL)}
	s.mu.Unlock()

	return n, nil
}

// Consume checks that n is the unexpired nonce on file for sn and, if so,
// deletes it so it cannot be used again. Returns false for a missing,
// mismatched, or expired nonce.
func (s *NonceStore) Consume(sn, n string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.m[sn]
	if !ok || entry.value != n {
		return false
	}
	delete(s.m, sn)
	if now.After(entry.expiresAt) {
		return false
	}
	return true
}
