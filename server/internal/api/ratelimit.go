package api

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/protocol"
)

// IPLimiter is a fixed-window request counter per client IP. Simple on
// purpose: the device-facing endpoints (protocol §8, code 1900) just need
// something that stops a runaway or misbehaving device from hammering the
// server, not a precise sliding-window algorithm — a fixed window that
// resets every WindowDur is enough, and doesn't need a background sweep to
// avoid leaking memory as long as ResetIfStale runs on every check.
type IPLimiter struct {
	mu          sync.Mutex
	limit       int
	windowDur   time.Duration
	windowStart time.Time
	counts      map[string]int
}

func NewIPLimiter(limit int, window time.Duration) *IPLimiter {
	return &IPLimiter{
		limit:       limit,
		windowDur:   window,
		windowStart: time.Now(),
		counts:      make(map[string]int),
	}
}

// Allow reports whether one more request from key should proceed,
// resetting all counts if the current window has elapsed.
func (l *IPLimiter) Allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.windowStart) >= l.windowDur {
		l.windowStart = now
		l.counts = make(map[string]int)
	}

	l.counts[key]++
	return l.counts[key] <= l.limit
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// withRateLimit wraps a device-facing handler, rejecting with the
// protocol's rate-limit code once the caller's IP exceeds limiter's
// budget for the current window. Not applied to the admin API — that's
// authenticated and low-volume by nature, a different threat model than
// an open, unauthenticated-until-activated device endpoint.
func withRateLimit(limiter *IPLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(clientIP(r), time.Now()) {
			writeErr(w, protocol.CodeRateLimited, "rate limit exceeded")
			return
		}
		next(w, r)
	}
}
