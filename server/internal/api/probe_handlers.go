package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/ubibot/ubibot-platform-open/internal/store"
)

// probeDTO is the operator-facing shape of a configured probe — Params is
// surfaced as a decoded map rather than a raw JSON string so the frontend
// doesn't need its own copy of the field-name conventions from protocol
// §7.2.
type probeDTO struct {
	Pid           string         `json:"pid"`
	Key           string         `json:"key"`
	Iface         string         `json:"iface"`
	Proto         string         `json:"proto"`
	Params        map[string]any `json:"params"`
	Status        string         `json:"status"`
	LastCommandID string         `json:"last_command_id,omitempty"`
	LastError     string         `json:"last_error,omitempty"`
}

// ListProbes handles GET /api/admin/devices/{id}/probes.
func (s *Server) ListProbes(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}

	probes, err := s.Store.ListProbes(uint(id))
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}

	list := make([]probeDTO, 0, len(probes))
	for _, p := range probes {
		var params map[string]any
		_ = decodeJSONString(p.Params, &params)
		list = append(list, probeDTO{
			Pid: p.Pid, Key: p.Key, Iface: p.Iface, Proto: p.Proto, Params: params,
			Status: p.Status, LastCommandID: p.LastCommandID, LastError: p.LastError,
		})
	}
	writeJSON(w, 200, map[string]any{"list": list})
}

type upsertProbeRequest struct {
	Pid    string         `json:"pid"`
	Key    string         `json:"key"`
	Iface  string         `json:"iface"`
	Proto  string         `json:"proto"`
	Params map[string]any `json:"params"`
}

// UpsertProbe handles POST /api/admin/devices/{id}/probes — this is the
// "探头自定义数据读取" configuration endpoint: it queues a set_probe
// command and marks the probe row "pending" until the device acks or naks
// it (see store.applyProbeCommandOutcome, wired in from report handling).
func (s *Server) UpsertProbe(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}

	if _, err := s.Store.DeviceByID(uint(id)); errors.Is(err, store.ErrNotFound) {
		adminErr(w, 404, "device not found")
		return
	}

	var req upsertProbeRequest
	if err := decodeJSON(r, &req); err != nil || req.Pid == "" || req.Iface == "" || req.Proto == "" {
		adminErr(w, 400, "pid, iface and proto are required")
		return
	}

	probe, cmd, err := s.Store.UpsertProbe(uint(id), store.ProbeInput{
		Pid: req.Pid, Key: req.Key, Iface: req.Iface, Proto: req.Proto, Params: req.Params,
	})
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "probe.upsert", "device", uint(id), req.Pid)

	var params map[string]any
	_ = decodeJSONString(probe.Params, &params)
	writeJSON(w, 200, map[string]any{
		"probe": probeDTO{
			Pid: probe.Pid, Key: probe.Key, Iface: probe.Iface, Proto: probe.Proto, Params: params,
			Status: probe.Status, LastCommandID: probe.LastCommandID, LastError: probe.LastError,
		},
		"command": toCommandDTO(cmd),
	})
}

// RemoveProbe handles DELETE /api/admin/devices/{id}/probes/{pid} — queues
// a set_probe(op=remove) command; the row itself only disappears once the
// device acks the removal.
func (s *Server) RemoveProbe(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	pid := r.PathValue("pid")
	if pid == "" {
		adminErr(w, 400, "pid is required")
		return
	}

	cmd, err := s.Store.RemoveProbe(uint(id), pid)
	if errors.Is(err, store.ErrNotFound) {
		adminErr(w, 404, "probe not found")
		return
	}
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "probe.remove", "device", uint(id), pid)
	writeJSON(w, 200, map[string]any{"command": toCommandDTO(cmd)})
}
