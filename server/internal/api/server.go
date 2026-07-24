// Package api implements the device-facing endpoints defined in
// docs/UbiBot开放平台硬件通信协议.md (time sync, data report — no
// activation, no session tokens, no command channel) plus a minimal admin
// API (login, device list/detail/rename/enable-disable/delete) backed by
// the same persistent store.
package api

import (
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/store"
)

// Server holds the dependencies the handlers need. Now is overridable so
// tests can control clock-dependent behaviour (the report ±5-minute
// timestamp window) deterministically.
type Server struct {
	Store       *store.Store
	RateLimiter *IPLimiter
	Now         func() time.Time

	// FileDir is where uploaded file assets are stored on disk; DBPath and
	// StartedAt back the 系统监控 metrics endpoint.
	FileDir   string
	DBPath    string
	StartedAt time.Time
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
		RateLimiter: NewIPLimiter(DefaultRateLimitPerMinute, time.Minute),
		Now:         time.Now,
		StartedAt:   time.Now(),
	}
}
