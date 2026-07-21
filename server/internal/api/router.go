package api

import "net/http"

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

	mux.HandleFunc("POST /api/v1/auth/time", s.TimeSync)
	mux.HandleFunc("POST /api/v1/auth/activate", s.Activate)
	mux.HandleFunc("POST /api/v1/data/report", s.Report)
	mux.HandleFunc("GET /api/v1/device/poll", s.Poll)

	mux.HandleFunc("POST /api/admin/login", s.AdminLogin)

	mux.HandleFunc("GET /api/admin/me", s.RequireAdmin(s.AdminMe))
	mux.HandleFunc("GET /api/admin/devices", s.RequireAdmin(s.ListDevices))
	mux.HandleFunc("POST /api/admin/devices", s.RequireAdmin(s.CreateDevice))
	mux.HandleFunc("GET /api/admin/devices/{id}", s.RequireAdmin(s.GetDevice))
	mux.HandleFunc("PATCH /api/admin/devices/{id}/config", s.RequireAdmin(s.UpdateDeviceConfig))
	mux.HandleFunc("POST /api/admin/devices/{id}/status", s.RequireAdmin(s.SetDeviceStatus))
	mux.HandleFunc("POST /api/admin/devices/{id}/commands", s.RequireAdmin(s.DispatchCommand))
	mux.HandleFunc("GET /api/admin/devices/{id}/commands", s.RequireAdmin(s.ListCommands))

	return withCORS(mux)
}
