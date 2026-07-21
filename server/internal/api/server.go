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
	Store  *store.Store
	Nonces *auth.NonceStore
	Now    func() time.Time
}

func NewServer(st *store.Store) *Server {
	return &Server{
		Store:  st,
		Nonces: auth.NewNonceStore(),
		Now:    time.Now,
	}
}
