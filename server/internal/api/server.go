// Package api implements the three device-facing HTTP endpoints defined in
// docs/UbiBot开放平台硬件通信协议.docx: time sync, activation, and data
// report with piggybacked config/command delivery.
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
	Tokens *auth.TokenStore
	Now    func() time.Time
}

func NewServer(st *store.Store) *Server {
	return &Server{
		Store:  st,
		Nonces: auth.NewNonceStore(),
		Tokens: auth.NewTokenStore(),
		Now:    time.Now,
	}
}
