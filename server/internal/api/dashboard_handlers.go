package api

import (
	"net/http"
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

// DashboardSummary handles GET /api/admin/dashboard/summary — the
// dashboard's stat cards, replacing the frontend's previous mock numbers
// with real aggregates.
func (s *Server) DashboardSummary(w http.ResponseWriter, r *http.Request) {
	devices, deviceTotal, err := s.Store.ListDevices(1, 500)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	now := s.Now()
	online := 0
	for i := range devices {
		if store.IsDeviceOnline(&devices[i], now) {
			online++
		}
	}

	_, openAlerts, err := s.Store.ListAlertEvents(store.AlertFilter{Status: model.AlertStatusOpen}, 1, 1)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	_, pendingCmds, err := s.Store.ListAllCommands(store.CommandFilter{Status: model.CommandStatusPending}, 1, 1)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	todayRecords, err := s.Store.CountRecordsSince(startOfToday)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}

	writeJSON(w, 200, map[string]any{
		"device_total":     deviceTotal,
		"device_online":    online,
		"open_alerts":      openAlerts,
		"pending_commands": pendingCmds,
		"today_records":    todayRecords,
	})
}

// DashboardTrends handles GET /api/admin/dashboard/trends — daily
// telemetry volume for the last 7 days, for a small trend chart.
func (s *Server) DashboardTrends(w http.ResponseWriter, r *http.Request) {
	since := s.Now().AddDate(0, 0, -6)
	sinceUnix := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, since.Location()).Unix()

	rows, err := s.Store.RecordCountsByDay(sinceUnix)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	writeJSON(w, 200, map[string]any{"days": rows})
}
