package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/ubibot/ubibot-platform-open/internal/model"
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

func currentAdmin(r *http.Request) *model.AdminUser {
	admin, _ := r.Context().Value(adminUserContextKey).(*model.AdminUser)
	return admin
}
