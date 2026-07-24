package api

import (
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

// SystemMetrics handles GET /api/admin/system/metrics — the 系统监控
// page's backing call. Deliberately cheap to compute (existing List*
// calls with page_size=1 just to read their total, rather than adding a
// parallel set of Count* methods) since this may be polled frequently.
func (s *Server) SystemMetrics(w http.ResponseWriter, r *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	_, deviceTotal, err := s.Store.ListDevices(1, 1)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	_, openAlerts, err := s.Store.ListAlertEvents(store.AlertFilter{Status: model.AlertStatusOpen}, 1, 1)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	unread, err := s.Store.CountUnreadNotifications()
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}

	var dbSize int64
	if info, err := os.Stat(s.DBPath); err == nil {
		dbSize = info.Size()
	}

	writeJSON(w, 200, map[string]any{
		"go_version":           runtime.Version(),
		"goroutines":           runtime.NumGoroutine(),
		"heap_alloc_bytes":     mem.HeapAlloc,
		"uptime_seconds":       int64(time.Since(s.StartedAt).Seconds()),
		"db_size_bytes":        dbSize,
		"device_total":         deviceTotal,
		"open_alerts":          openAlerts,
		"unread_notifications": unread,
	})
}
