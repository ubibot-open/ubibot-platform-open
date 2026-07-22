package api

import (
	"io/fs"
	"net/http"
	"strings"

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

// spaHandler serves the embedded admin frontend build: real static assets
// (JS/CSS/images) are served as-is, and everything else falls back to
// index.html so client-side routes (React Router paths like
// /data-warehouse/12) resolve correctly on a hard refresh instead of 404ing
// against the Go server before React ever gets a chance to route them.
func spaHandler(ui fs.FS) http.Handler {
	fileServer := http.FileServerFS(ui)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if f, err := ui.Open(p); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFileFS(w, r, ui, "index.html")
	})
}

// NewRouter wires the device-facing endpoints (protocol §4/§5/§7.1) and
// the admin API (login + device/command management) onto a stdlib
// ServeMux, using Go 1.22's method+pattern routing ("POST /path") and
// {name} path parameters instead of a web framework. ui is the embedded
// admin frontend build (see internal/webui) — pass a nil ui / uiBuilt=false
// to run API-only (e.g. in tests, or before the frontend has ever been
// built), in which case nothing is mounted at "/".
func NewRouter(s *Server, ui fs.FS, uiBuilt bool) http.Handler {
	mux := http.NewServeMux()

	// Device-facing endpoints (protocol §4/§5/§7.1) are open to anyone who
	// can reach the server (auth happens inside via signature/token, not
	// at the transport layer), so these — and only these — get the IP
	// rate limiter (protocol §8, code 1900).
	mux.HandleFunc("POST /api/v1/auth/time", withRateLimit(s.RateLimiter, s.TimeSync))
	mux.HandleFunc("POST /api/v1/auth/activate", withRateLimit(s.RateLimiter, s.Activate))
	mux.HandleFunc("POST /api/v1/data/report", withRateLimit(s.RateLimiter, s.Report))
	mux.HandleFunc("GET /api/v1/device/poll", withRateLimit(s.RateLimiter, s.Poll))
	mux.HandleFunc("GET /api/v1/ota/firmware", withRateLimit(s.RateLimiter, s.OtaFirmwareDownload))

	mux.HandleFunc("POST /api/admin/login", s.AdminLogin)
	mux.HandleFunc("GET /api/admin/me", s.RequireAdmin(s.AdminMe))

	// Device read/write.
	mux.HandleFunc("GET /api/admin/devices", s.RequirePermission(model.PermDeviceRead, s.ListDevices))
	// "数据仓库" (data warehouse): activated devices only, each with its
	// latest telemetry record inlined -- registered before the {id} routes
	// below purely for readability, Go 1.22's mux dispatches by exact
	// literal-vs-wildcard segment so "data-warehouse" never matches {id}.
	mux.HandleFunc("GET /api/admin/devices/data-warehouse", s.RequirePermission(model.PermDeviceRead, s.ListDataWarehouse))
	mux.HandleFunc("POST /api/admin/devices", s.RequirePermission(model.PermDeviceWrite, s.CreateDevice))
	mux.HandleFunc("GET /api/admin/devices/{id}", s.RequirePermission(model.PermDeviceRead, s.GetDevice))
	mux.HandleFunc("GET /api/admin/devices/{id}/records", s.RequirePermission(model.PermDeviceRead, s.GetDeviceRecords))
	mux.HandleFunc("PATCH /api/admin/devices/{id}/config", s.RequirePermission(model.PermDeviceWrite, s.UpdateDeviceConfig))
	mux.HandleFunc("POST /api/admin/devices/{id}/status", s.RequirePermission(model.PermDeviceWrite, s.SetDeviceStatus))
	// Approve a Pending, self-registered device (model.DeviceSourceSelfRegistered)
	// -- see api.Activate/api.ApproveDevice. Rejecting one, and disabling/
	// re-enabling an already-approved device, both reuse the status route
	// above (POST .../status with DeviceStatusDisabled/Enabled) rather than
	// needing dedicated endpoints of their own.
	mux.HandleFunc("POST /api/admin/devices/{id}/approve", s.RequirePermission(model.PermDeviceWrite, s.ApproveDevice))
	mux.HandleFunc("DELETE /api/admin/devices/{id}", s.RequirePermission(model.PermDeviceWrite, s.DeleteDevice))

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

	// OTA (protocol §7.3): firmware asset management rides on system:manage
	// (a shared asset, not scoped to one device); per-device dispatch/
	// cancel/status ride on the same permissions as command dispatch and
	// device read.
	mux.HandleFunc("GET /api/admin/firmware", s.RequirePermission(model.PermSystemManage, s.ListFirmware))
	mux.HandleFunc("POST /api/admin/firmware", s.RequirePermission(model.PermSystemManage, s.UploadFirmware))
	mux.HandleFunc("DELETE /api/admin/firmware/{id}", s.RequirePermission(model.PermSystemManage, s.DeleteFirmware))
	mux.HandleFunc("GET /api/admin/devices/{id}/ota", s.RequirePermission(model.PermDeviceRead, s.GetDeviceOTA))
	mux.HandleFunc("POST /api/admin/devices/{id}/ota", s.RequirePermission(model.PermCommandWrite, s.DispatchDeviceOTA))
	mux.HandleFunc("POST /api/admin/devices/{id}/ota/cancel", s.RequirePermission(model.PermCommandWrite, s.CancelDeviceOTA))

	// 消息中心.
	mux.HandleFunc("GET /api/admin/notifications", s.RequireAdmin(s.ListNotifications))
	mux.HandleFunc("POST /api/admin/notifications/{id}/read", s.RequireAdmin(s.MarkNotificationRead))
	mux.HandleFunc("POST /api/admin/notifications/read-all", s.RequireAdmin(s.MarkAllNotificationsRead))

	// 定时任务.
	mux.HandleFunc("GET /api/admin/scheduled-tasks", s.RequirePermission(model.PermCommandWrite, s.ListScheduledTasks))
	mux.HandleFunc("POST /api/admin/scheduled-tasks", s.RequirePermission(model.PermCommandWrite, s.CreateScheduledTask))
	mux.HandleFunc("PATCH /api/admin/scheduled-tasks/{id}", s.RequirePermission(model.PermCommandWrite, s.UpdateScheduledTask))
	mux.HandleFunc("DELETE /api/admin/scheduled-tasks/{id}", s.RequirePermission(model.PermCommandWrite, s.DeleteScheduledTask))

	// 开放API管理 + 只读对外接口.
	mux.HandleFunc("GET /api/admin/api-keys", s.RequirePermission(model.PermSystemManage, s.ListApiKeys))
	mux.HandleFunc("POST /api/admin/api-keys", s.RequirePermission(model.PermSystemManage, s.CreateApiKey))
	mux.HandleFunc("POST /api/admin/api-keys/{id}/revoke", s.RequirePermission(model.PermSystemManage, s.RevokeApiKey))
	mux.HandleFunc("GET /api/open/v1/devices", s.RequireApiKey(s.OpenListDevices))
	mux.HandleFunc("GET /api/open/v1/devices/{id}/records", s.RequireApiKey(s.OpenGetDeviceRecords))

	// 文件/字典/参数.
	mux.HandleFunc("GET /api/admin/files", s.RequirePermission(model.PermSystemManage, s.ListFileAssets))
	mux.HandleFunc("POST /api/admin/files", s.RequirePermission(model.PermSystemManage, s.UploadFileAsset))
	mux.HandleFunc("DELETE /api/admin/files/{id}", s.RequirePermission(model.PermSystemManage, s.DeleteFileAsset))
	mux.HandleFunc("GET /api/admin/dict", s.RequireAdmin(s.ListDictEntries))
	mux.HandleFunc("POST /api/admin/dict", s.RequirePermission(model.PermSystemManage, s.CreateDictEntry))
	mux.HandleFunc("PATCH /api/admin/dict/{id}", s.RequirePermission(model.PermSystemManage, s.UpdateDictEntry))
	mux.HandleFunc("DELETE /api/admin/dict/{id}", s.RequirePermission(model.PermSystemManage, s.DeleteDictEntry))
	mux.HandleFunc("GET /api/admin/params", s.RequirePermission(model.PermSystemManage, s.ListSystemParams))
	mux.HandleFunc("PATCH /api/admin/params/{key}", s.RequirePermission(model.PermSystemManage, s.SetSystemParam))

	// 图标库 (数据仓库传感器图标覆盖) — list rides on device:read since it
	// only affects how telemetry is displayed; upload/delete are a system
	// asset change like files/dict, so they ride on system:manage.
	mux.HandleFunc("GET /api/admin/icons", s.RequirePermission(model.PermDeviceRead, s.ListIcons))
	mux.HandleFunc("POST /api/admin/icons", s.RequirePermission(model.PermSystemManage, s.UploadIcon))
	mux.HandleFunc("DELETE /api/admin/icons/{key}", s.RequirePermission(model.PermSystemManage, s.DeleteIcon))

	// 系统监控 + 仪表盘.
	mux.HandleFunc("GET /api/admin/system/metrics", s.RequirePermission(model.PermSystemManage, s.SystemMetrics))
	mux.HandleFunc("GET /api/admin/dashboard/summary", s.RequireAdmin(s.DashboardSummary))
	mux.HandleFunc("GET /api/admin/dashboard/trends", s.RequireAdmin(s.DashboardTrends))

	// The embedded admin frontend, if this binary was built with one (see
	// internal/webui) — registered last / on the catch-all "/" pattern so
	// it never shadows any /api/... route above regardless of registration
	// order (Go 1.22's mux prefers the most specific pattern anyway, but
	// being last here also just reads correctly).
	if uiBuilt {
		mux.Handle("/", spaHandler(ui))
	}

	return withCORS(mux)
}
