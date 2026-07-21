package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

type contextKey int

const adminUserContextKey contextKey = iota

func adminErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"message": msg})
}

// RequireAdmin wraps next, checking the Authorization: Bearer <token>
// header against the admin_sessions table before calling through. Valid
// sessions get the admin account stashed on the request context for
// currentAdmin to read.
func (s *Server) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token := strings.TrimPrefix(header, "Bearer ")
		if token == "" || token == header {
			adminErr(w, 401, "missing bearer token")
			return
		}

		admin, err := s.Store.ValidateAdminSession(token)
		if err != nil {
			adminErr(w, 401, "invalid or expired session")
			return
		}

		ctx := context.WithValue(r.Context(), adminUserContextKey, admin)
		next(w, r.WithContext(ctx))
	}
}

// RequirePermission builds on RequireAdmin, additionally checking that
// the logged-in admin's role (see model.Role, store.HasPermission) grants
// code — this is the RBAC gate every mutating (and a few sensitive
// read) admin endpoint goes through.
func (s *Server) RequirePermission(code string, next http.HandlerFunc) http.HandlerFunc {
	return s.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		admin := currentAdmin(r)
		role, err := s.Store.AdminRole(admin)
		if err != nil {
			adminErr(w, 500, "internal error")
			return
		}
		if !store.HasPermission(role, code) {
			adminErr(w, 403, "forbidden")
			return
		}
		next(w, r)
	})
}

// RequireApiKey gates the read-only /api/open/v1 surface — a separate
// credential from admin sessions (RequireAdmin) and device tokens,
// carried in X-Api-Key rather than Authorization so it can't be confused
// with either.
func (s *Server) RequireApiKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Api-Key")
		if key == "" {
			adminErr(w, 401, "missing api key")
			return
		}
		if _, err := s.Store.ValidateApiKey(key); err != nil {
			adminErr(w, 401, "invalid or revoked api key")
			return
		}
		next(w, r)
	}
}

func currentAdmin(r *http.Request) *model.AdminUser {
	admin, _ := r.Context().Value(adminUserContextKey).(*model.AdminUser)
	return admin
}

// audit records a mutating action against the currently-authenticated
// admin (a no-op if called from an unauthenticated context, which
// shouldn't happen since every caller sits behind RequireAdmin/
// RequirePermission). Failure to write the log is logged, not surfaced —
// an audit-log outage shouldn't block the action it's trying to record.
func (s *Server) audit(r *http.Request, action, targetType string, targetID uint, detail string) {
	admin := currentAdmin(r)
	if admin == nil {
		return
	}
	_ = s.Store.WriteAudit(admin.ID, admin.Username, action, targetType, targetID, detail, clientIP(r))
}
