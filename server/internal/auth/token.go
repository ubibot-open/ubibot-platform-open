package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// TokenTTL is the default session token validity period (86400s per the doc).
const TokenTTL = 24 * time.Hour

// RenewThreshold: devices are expected to proactively re-activate once the
// remaining validity drops below this (see X-Token-Expires-In); the server
// side doesn't need to act on it, it's just what ExpiresIn communicates.
const RenewThreshold = time.Hour

type tokenEntry struct {
	did       string
	expiresAt time.Time
}

// TokenStore issues and validates opaque bearer tokens.
type TokenStore struct {
	mu sync.Mutex
	m  map[string]tokenEntry
}

func NewTokenStore() *TokenStore {
	return &TokenStore{m: make(map[string]tokenEntry)}
}

// Issue creates a new token for did, valid for TokenTTL from now.
func (s *TokenStore) Issue(did string, now time.Time) (token string, exp time.Duration, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", 0, err
	}
	token = hex.EncodeToString(buf)

	s.mu.Lock()
	s.m[token] = tokenEntry{did: did, expiresAt: now.Add(TokenTTL)}
	s.mu.Unlock()

	return token, TokenTTL, nil
}

// TokenStatus is the result of validating a token.
type TokenStatus int

const (
	TokenValid TokenStatus = iota
	TokenNotFound
	TokenExpired
)

// Validate looks up token and reports its status plus, when valid, the
// owning device id and remaining validity.
func (s *TokenStore) Validate(token string, now time.Time) (did string, remaining time.Duration, status TokenStatus) {
	s.mu.Lock()
	entry, ok := s.m[token]
	s.mu.Unlock()

	if !ok {
		return "", 0, TokenNotFound
	}
	if now.After(entry.expiresAt) {
		return "", 0, TokenExpired
	}
	return entry.did, entry.expiresAt.Sub(now), TokenValid
}

// Revoke removes a token, e.g. after the device reports it as lost.
func (s *TokenStore) Revoke(token string) {
	s.mu.Lock()
	delete(s.m, token)
	s.mu.Unlock()
}
