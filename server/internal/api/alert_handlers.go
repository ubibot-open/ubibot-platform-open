package api

import (
	"net/http"
	"strconv"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

type alertRuleDTO struct {
	ID        uint    `json:"id"`
	DeviceID  uint    `json:"device_id"`
	Field     string  `json:"field"`
	Op        string  `json:"op"`
	Threshold float64 `json:"threshold"`
	Enabled   bool    `json:"enabled"`
}

func toAlertRuleDTO(r *model.AlertRule) alertRuleDTO {
	return alertRuleDTO{ID: r.ID, DeviceID: r.DeviceID, Field: r.Field, Op: r.Op, Threshold: r.Threshold, Enabled: r.Enabled}
}

// ListAlertRules handles GET /api/admin/devices/{id}/alert-rules.
func (s *Server) ListAlertRules(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	rules, err := s.Store.ListAlertRules(uint(id))
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	list := make([]alertRuleDTO, 0, len(rules))
	for i := range rules {
		list = append(list, toAlertRuleDTO(&rules[i]))
	}
	writeJSON(w, 200, map[string]any{"list": list})
}

type createAlertRuleRequest struct {
	Field     string  `json:"field"`
	Op        string  `json:"op"`
	Threshold float64 `json:"threshold"`
}

var validAlertOps = map[string]bool{
	model.AlertOpGT: true, model.AlertOpGE: true, model.AlertOpLT: true,
	model.AlertOpLE: true, model.AlertOpEQ: true,
}

// CreateAlertRule handles POST /api/admin/devices/{id}/alert-rules — the
// "阈值告警" configuration endpoint (offline alerting has no rule row; see
// store.OfflineSweep).
func (s *Server) CreateAlertRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	var req createAlertRuleRequest
	if err := decodeJSON(r, &req); err != nil || req.Field == "" || !validAlertOps[req.Op] {
		adminErr(w, 400, "field and a valid op (> >= < <= ==) are required")
		return
	}

	rule, err := s.Store.CreateAlertRule(uint(id), req.Field, req.Op, req.Threshold)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "alert_rule.create", "device", uint(id), req.Field+" "+req.Op)
	writeJSON(w, 200, toAlertRuleDTO(rule))
}

// DeleteAlertRule handles DELETE /api/admin/alert-rules/{id}.
func (s *Server) DeleteAlertRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	if err := s.Store.DeleteAlertRule(uint(id)); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "alert_rule.delete", "alert_rule", uint(id), "")
	writeJSON(w, 200, map[string]any{"message": "ok"})
}

type alertEventDTO struct {
	ID          uint   `json:"id"`
	DeviceID    uint   `json:"device_id"`
	DeviceName  string `json:"device_name"`
	RuleID      uint   `json:"rule_id"`
	Type        string `json:"type"`
	Message     string `json:"message"`
	Status      string `json:"status"`
	TriggeredAt int64  `json:"triggered_at"`
	ResolvedAt  *int64 `json:"resolved_at"`
}

// ListAlertEvents handles GET /api/admin/alert-events — the "告警中心"
// page's backing list, filterable by device_id/status.
func (s *Server) ListAlertEvents(w http.ResponseWriter, r *http.Request) {
	page, pageSize := paginationParams(r)
	deviceID, _ := strconv.Atoi(r.URL.Query().Get("device_id"))

	events, total, err := s.Store.ListAlertEvents(store.AlertFilter{
		DeviceID: uint(deviceID),
		Status:   r.URL.Query().Get("status"),
	}, page, pageSize)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}

	deviceNames := make(map[uint]string)
	list := make([]alertEventDTO, 0, len(events))
	for _, ev := range events {
		name, ok := deviceNames[ev.DeviceID]
		if !ok {
			if dev, err := s.Store.DeviceByID(ev.DeviceID); err == nil {
				name = dev.Name
				if name == "" {
					name = dev.SN
				}
			}
			deviceNames[ev.DeviceID] = name
		}
		dto := alertEventDTO{
			ID: ev.ID, DeviceID: ev.DeviceID, DeviceName: name, RuleID: ev.RuleID,
			Type: ev.Type, Message: ev.Message, Status: ev.Status,
			TriggeredAt: ev.TriggeredAt.Unix(),
		}
		if ev.ResolvedAt != nil {
			t := ev.ResolvedAt.Unix()
			dto.ResolvedAt = &t
		}
		list = append(list, dto)
	}
	writeJSON(w, 200, map[string]any{"list": list, "total": total})
}

// ResolveAlertEvent handles POST /api/admin/alert-events/{id}/resolve.
func (s *Server) ResolveAlertEvent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	if err := s.Store.ResolveAlertEvent(uint(id)); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "alert_event.resolve", "alert_event", uint(id), "")
	writeJSON(w, 200, map[string]any{"message": "ok"})
}
