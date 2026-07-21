package api

import (
	"net/http"

	"github.com/ubibot/ubibot-platform-open/internal/model"
)

// withCORS allows the admin frontend (served from a different origin in
// dev, e.g. http://localhost:5173) to call this API. Auth is a bearer
// token in a header, not a cookie, so a wildcard origin carries no CSRF
// risk here — tightening this to a configured origin list is a
// deploy-time concern, not a P0 one.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-IoT-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// NewRouter wires the device-facing endpoints (protocol §4/§5/§7.1) and
// the admin API (login + device/command management) onto a stdlib
// ServeMux, using Go 1.22's method+pattern routing ("POST /path") and
// {name} path parameters instead of a web framework.
func NewRouter(s *Server) http.Handler {
	mux := http.NewServeMux()

	// Device-facing endpoints (protocol §4/§5/§7.1) are open to anyone who
	// can reach the server (auth happens inside via signature/token, not
	// at the transport layer), so these — and only these — get the IP
	// rate limiter (protocol §8, code 1900).
	mux.HandleFunc("POST /api/v1/auth/time", withRateLimit(s.RateLimiter, s.TimeSync))
	mux.HandleFunc("POST /api/v1/auth/activate", withRateLimit(s.RateLimiter, s.Activate))
	mux.HandleFunc("POST /api/v1/data/report", withRateLimit(s.RateLimiter, s.Report))
	mux.HandleFunc("GET /api/v1/device/poll", withRateLimit(s.RateLimiter, s.Poll))

	mux.HandleFunc("POST /api/admin/login", s.AdminLogin)
	mux.HandleFunc("GET /api/admin/me", s.RequireAdmin(s.AdminMe))

	// Device read/write.
	mux.HandleFunc("GET /api/admin/devices", s.RequirePermission(model.PermDeviceRead, s.ListDevices))
	mux.HandleFunc("POST /api/admin/devices", s.RequirePermission(model.PermDeviceWrite, s.CreateDevice))
	mux.HandleFunc("GET /api/admin/devices/{id}", s.RequirePermission(model.PermDeviceRead, s.GetDevice))
	mux.HandleFunc("GET /api/admin/devices/{id}/records", s.RequirePermission(model.PermDeviceRead, s.GetDeviceRecords))
	mux.HandleFunc("PATCH /api/admin/devices/{id}/config", s.RequirePermission(model.PermDeviceWrite, s.UpdateDeviceConfig))
	mux.HandleFunc("POST /api/admin/devices/{id}/status", s.RequirePermission(model.PermDeviceWrite, s.SetDeviceStatus))

	// Probe configuration (protocol §7.2 set_probe) rides on device:write —
	// it's a device configuration change, same as UpdateDeviceConfig.
	mux.HandleFunc("GET /api/admin/devices/{id}/probes", s.RequirePermission(model.PermDeviceRead, s.ListProbes))
	mux.HandleFunc("POST /api/admin/devices/{id}/probes", s.RequirePermission(model.PermDeviceWrite, s.UpsertProbe))
	mux.HandleFunc("DELETE /api/admin/devices/{id}/probes/{pid}", s.RequirePermission(model.PermDeviceWrite, s.RemoveProbe))

	// Command dispatch/history.
	mux.HandleFunc("POST /api/admin/devices/{id}/commands", s.RequirePermission(model.PermCommandWrite, s.DispatchCommand))
	mux.HandleFunc("GET /api/admin/devices/{id}/commands", s.RequirePermission(model.PermDeviceRead, s.ListCommands))
	mux.HandleFunc("GET /api/admin/commands", s.RequirePermission(model.PermDeviceRead, s.ListAllCommands))

	// Alerting.
	mux.HandleFunc("GET /api/admin/devices/{id}/alert-rules", s.RequirePermission(model.PermDeviceRead, s.ListAlertRules))
	mux.HandleFunc("POST /api/admin/devices/{id}/alert-rules", s.RequirePermission(model.PermAlertManage, s.CreateAlertRule))
	mux.HandleFunc("DELETE /api/admin/alert-rules/{id}", s.RequirePermission(model.PermAlertManage, s.DeleteAlertRule))
	mux.HandleFunc("GET /api/admin/alert-events", s.RequirePermission(model.PermDeviceRead, s.ListAlertEvents))
	mux.HandleFunc("POST /api/admin/alert-events/{id}/resolve", s.RequirePermission(model.PermAlertManage, s.ResolveAlertEvent))

	// RBAC and audit — system administration.
	mux.HandleFunc("GET /api/admin/roles", s.RequirePermission(model.PermSystemManage, s.ListRoles))
	mux.HandleFunc("POST /api/admin/roles", s.RequirePermission(model.PermSystemManage, s.CreateRole))
	mux.HandleFunc("PATCH /api/admin/roles/{id}", s.RequirePermission(model.PermSystemManage, s.UpdateRole))
	mux.HandleFunc("DELETE /api/admin/roles/{id}", s.RequirePermission(model.PermSystemManage, s.DeleteRole))
	mux.HandleFunc("GET /api/admin/users", s.RequirePermission(model.PermSystemManage, s.ListAdminUsers))
	mux.HandleFunc("POST /api/admin/users", s.RequirePermission(model.PermSystemManage, s.CreateAdminUser))
	mux.HandleFunc("PATCH /api/admin/users/{id}", s.RequirePermission(model.PermSystemManage, s.UpdateAdminUser))
	mux.HandleFunc("DELETE /api/admin/users/{id}", s.RequirePermission(model.PermSystemManage, s.DeleteAdminUser))
	mux.HandleFunc("GET /api/admin/audit-logs", s.RequirePermission(model.PermSystemManage, s.ListAuditLogs))

	return withCORS(mux)
}
