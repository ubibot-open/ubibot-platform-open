// Package api implements the device-facing endpoints defined in
// docs/UbiBot开放平台硬件通信协议.md (time sync, activation, data report
// with piggybacked config/command delivery) plus a minimal admin API
// (login, device list/detail, manual command dispatch) backed by the
// same persistent store.
package api

import (
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/auth"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

// Server holds the dependencies the handlers need. Now is overridable so
// tests can control clock-dependent behaviour (nonce/token expiry, the
// ±5-minute activation window) deterministically.
type Server struct {
	Store       *store.Store
	Nonces      *auth.NonceStore
	RateLimiter *IPLimiter
	Now         func() time.Time

	// FirmwareDir is where uploaded OTA images (protocol §7.3) are stored
	// on disk; DBPath and StartedAt back the 系统监控 metrics endpoint.
	FirmwareDir string
	FileDir     string
	DBPath      string
	StartedAt   time.Time

	// ServerKeyPair is this platform instance's RSA keypair (docs §4.1) —
	// a self-registration provisioning tool encrypts a device's real
	// secret against its public half before submitting it to
	// POST /api/v1/auth/bind-key; only this process's private half can
	// decrypt it back (see PublicKey/BindDeviceKey). Nil is a valid zero
	// value for tests that don't exercise that endpoint.
	ServerKeyPair *auth.KeyPairPEM
}

// DefaultRateLimitPerMinute is how many device-facing requests a single
// IP may make per minute before getting 429/1900 — generous enough for a
// device reporting every few seconds plus retries, tight enough to blunt
// a runaway loop or someone scripting against the endpoint. Overridable
// at runtime via the "rate_limit_per_minute" system parameter.
const DefaultRateLimitPerMinute = 120

func NewServer(st *store.Store) *Server {
	return &Server{
		Store:       st,
		Nonces:      auth.NewNonceStore(),
		RateLimiter: NewIPLimiter(DefaultRateLimitPerMinute, time.Minute),
		Now:         time.Now,
		StartedAt:   time.Now(),
	}
}
