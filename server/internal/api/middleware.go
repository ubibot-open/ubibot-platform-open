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

// adminErrCodes maps every known adminErr message text to a stable,
// language-neutral code. The admin API intentionally does not localize
// message text itself (see docs on i18n) -- it returns this code alongside
// the English message, and the admin frontend translates by code, falling
// back to the English message verbatim for any code it doesn't recognize
// (e.g. a message passed through from err.Error() that isn't in this
// table, which maps to the generic "error" code below).
var adminErrCodes = map[string]string{
	"cannot delete your own account":                    "cannot_delete_own_account",
	"device not found":                                  "device_not_found",
	"failed to store file":                              "file_store_failed",
	"field and a valid op (> >= < <= ==) are required":  "invalid_field_or_operator",
	"file is required":                                  "file_required",
	"file not found":                                    "file_not_found",
	"forbidden":                                          "forbidden",
	"internal error":                                     "internal_error",
	"invalid id":                                         "invalid_id",
	"invalid key":                                        "invalid_key",
	"invalid or expired session":                         "session_invalid_or_expired",
	"invalid or revoked api key":                         "api_key_invalid_or_revoked",
	"invalid request body":                               "invalid_request_body",
	"invalid status":                                     "invalid_status",
	"invalid upload (file too large or malformed)":       "invalid_upload",
	"invalid username or password":                       "invalid_credentials",
	"label is required":                                  "label_required",
	"missing api key":                                    "api_key_missing",
	"missing bearer token":                               "bearer_token_missing",
	"name and code are required":                         "name_code_required",
	"name is required":                                   "name_required",
	"role already exists or invalid input":                "role_exists_or_invalid",
	"status is required":                                  "status_required",
	"type is required":                                    "type_required",
	"type, key and label are required":                    "type_key_label_required",
	"username already exists or invalid input":            "username_exists_or_invalid",
	"username and password are required":                  "username_password_required",
	"username, password and role_id are required":         "username_password_role_required",
	"value is required":                                   "value_required",
}

// messageCode returns the stable i18n code for msg, or the generic "error"
// fallback for a message not in adminErrCodes (currently only reachable via
// the one call site that passes through a dynamic err.Error()).
func messageCode(msg string) string {
	if code, ok := adminErrCodes[msg]; ok {
		return code
	}
	return "error"
}

func adminErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"code": messageCode(msg), "message": msg})
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
