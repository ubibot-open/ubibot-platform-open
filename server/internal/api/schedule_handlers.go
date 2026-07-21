package api

import (
	"net/http"
	"strconv"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

type scheduledTaskDTO struct {
	ID              uint           `json:"id"`
	Name            string         `json:"name"`
	DeviceID        uint           `json:"device_id"`
	CmdType         string         `json:"cmd_type"`
	CmdArgs         map[string]any `json:"cmd_args,omitempty"`
	ScheduleType    string         `json:"schedule_type"`
	IntervalSeconds int            `json:"interval_seconds,omitempty"`
	DailyAtMinute   int            `json:"daily_at_minute,omitempty"`
	Enabled         bool           `json:"enabled"`
	NextRunAt       int64          `json:"next_run_at"`
	LastRunAt       *int64         `json:"last_run_at"`
}

func toScheduledTaskDTO(t *model.ScheduledTask) scheduledTaskDTO {
	dto := scheduledTaskDTO{
		ID: t.ID, Name: t.Name, DeviceID: t.DeviceID, CmdType: t.CmdType,
		ScheduleType: t.ScheduleType, IntervalSeconds: t.IntervalSeconds, DailyAtMinute: t.DailyAtMinute,
		Enabled: t.Enabled, NextRunAt: t.NextRunAt.Unix(),
	}
	if t.CmdArgs != "" {
		var args map[string]any
		if err := decodeJSONString(t.CmdArgs, &args); err == nil {
			dto.CmdArgs = args
		}
	}
	if t.LastRunAt != nil {
		v := t.LastRunAt.Unix()
		dto.LastRunAt = &v
	}
	return dto
}

// ListScheduledTasks handles GET /api/admin/scheduled-tasks.
func (s *Server) ListScheduledTasks(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListScheduledTasks()
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	list := make([]scheduledTaskDTO, 0, len(rows))
	for i := range rows {
		list = append(list, toScheduledTaskDTO(&rows[i]))
	}
	writeJSON(w, 200, map[string]any{"list": list})
}

type scheduledTaskRequest struct {
	Name            string         `json:"name"`
	DeviceID        uint           `json:"device_id"`
	CmdType         string         `json:"cmd_type"`
	CmdArgs         map[string]any `json:"cmd_args"`
	ScheduleType    string         `json:"schedule_type"`
	IntervalSeconds int            `json:"interval_seconds"`
	DailyAtMinute   int            `json:"daily_at_minute"`
	Enabled         bool           `json:"enabled"`
}

func (req scheduledTaskRequest) valid() bool {
	if req.Name == "" || req.CmdType == "" {
		return false
	}
	switch req.ScheduleType {
	case model.ScheduleTypeInterval:
		return req.IntervalSeconds > 0
	case model.ScheduleTypeDaily:
		return req.DailyAtMinute >= 0 && req.DailyAtMinute < 24*60
	default:
		return false
	}
}

// CreateScheduledTask handles POST /api/admin/scheduled-tasks.
func (s *Server) CreateScheduledTask(w http.ResponseWriter, r *http.Request) {
	var req scheduledTaskRequest
	if err := decodeJSON(r, &req); err != nil || !req.valid() {
		adminErr(w, 400, "name, cmd_type and a valid schedule are required")
		return
	}
	t, err := s.Store.CreateScheduledTask(store.ScheduledTaskInput{
		Name: req.Name, DeviceID: req.DeviceID, CmdType: req.CmdType, CmdArgs: req.CmdArgs,
		ScheduleType: req.ScheduleType, IntervalSeconds: req.IntervalSeconds, DailyAtMinute: req.DailyAtMinute,
		Enabled: req.Enabled,
	})
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "scheduled_task.create", "scheduled_task", t.ID, req.Name)
	writeJSON(w, 200, toScheduledTaskDTO(t))
}

// UpdateScheduledTask handles PATCH /api/admin/scheduled-tasks/{id}.
func (s *Server) UpdateScheduledTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	var req scheduledTaskRequest
	if err := decodeJSON(r, &req); err != nil || !req.valid() {
		adminErr(w, 400, "name, cmd_type and a valid schedule are required")
		return
	}
	if err := s.Store.UpdateScheduledTask(uint(id), store.ScheduledTaskInput{
		Name: req.Name, DeviceID: req.DeviceID, CmdType: req.CmdType, CmdArgs: req.CmdArgs,
		ScheduleType: req.ScheduleType, IntervalSeconds: req.IntervalSeconds, DailyAtMinute: req.DailyAtMinute,
		Enabled: req.Enabled,
	}); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "scheduled_task.update", "scheduled_task", uint(id), "")
	writeJSON(w, 200, map[string]any{"message": "ok"})
}

// DeleteScheduledTask handles DELETE /api/admin/scheduled-tasks/{id}.
func (s *Server) DeleteScheduledTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	if err := s.Store.DeleteScheduledTask(uint(id)); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "scheduled_task.delete", "scheduled_task", uint(id), "")
	writeJSON(w, 200, map[string]any{"message": "ok"})
}
