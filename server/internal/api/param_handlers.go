package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

type systemParamDTO struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

func toSystemParamDTO(p *model.SystemParam) systemParamDTO {
	return systemParamDTO{Key: p.Key, Value: p.Value, Description: p.Description}
}

// ListSystemParams handles GET /api/admin/params.
func (s *Server) ListSystemParams(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListParams()
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	list := make([]systemParamDTO, 0, len(rows))
	for i := range rows {
		list = append(list, toSystemParamDTO(&rows[i]))
	}
	writeJSON(w, 200, map[string]any{"list": list})
}

type setSystemParamRequest struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}

// SetSystemParam handles PATCH /api/admin/params/{key}. A handful of keys
// (see ApplyParam) take effect immediately against live server state —
// not just recorded for the next restart to pick up.
func (s *Server) SetSystemParam(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		adminErr(w, 400, "invalid key")
		return
	}
	var req setSystemParamRequest
	if err := decodeJSON(r, &req); err != nil || req.Value == "" {
		adminErr(w, 400, "value is required")
		return
	}
	p, err := s.Store.SetParam(key, req.Value, req.Description)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.ApplyParam(key, req.Value)
	s.audit(r, "param.set", "system_param", 0, key+"="+req.Value)
	writeJSON(w, 200, toSystemParamDTO(p))
}

// ApplyParam pushes a system parameter's value into live server state for
// the keys that actually change runtime behavior. Called both right after
// an admin edits a parameter and once at startup (see cmd/server/main.go)
// after seeding defaults — the DB row is always the source of truth, this
// just keeps in-memory state (the rate limiter, the offline-grace
// override) in sync with it.
func (s *Server) ApplyParam(key, value string) {
	switch key {
	case store.ParamRateLimitPerMinute:
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			s.RateLimiter.SetLimit(n)
		}
	case store.ParamOfflineGraceMinute:
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			store.SetMinOfflineGrace(time.Duration(n) * time.Minute)
		}
	}
}
