package api

import (
	"net/http"
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/protocol"
)

// TimeWindow is the tolerance used to validate a report's ts against the
// server clock (docs §7, code 1002) — the only "freshness" check left in
// this protocol now that there's no signature or nonce to anchor to.
const TimeWindow = 5 * time.Minute

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, protocol.HTTPStatusFor(code), protocol.ErrorResponse{C: code, M: msg})
}

// TimeSync handles POST /api/v1/auth/time (docs §3) — a convenience for a
// device with no local clock reference yet. Deliberately unauthenticated:
// it reveals nothing about any device, so there's no reason to check
// pid/sn against anything.
func (s *Server) TimeSync(w http.ResponseWriter, r *http.Request) {
	var req protocol.TimeSyncRequest
	if err := decodeJSON(r, &req); err != nil || req.PID == "" || req.SN == "" {
		writeErr(w, protocol.CodeMalformedBody, "malformed request body")
		return
	}
	writeJSON(w, 200, protocol.TimeSyncResponse{C: protocol.CodeOK, T: s.Now().Unix()})
}

// Report handles POST /api/v1/data/report (docs §4) — the device's only
// other endpoint. There is no secret, signature, or token: pid+sn in the
// body is the entire identity. A device reporting an SN this platform has
// never seen is auto-created on the spot (see store.GetOrCreateDeviceBySN)
// — the whole point of this protocol is that there's no separate
// provisioning/activation step to get in the way of that.
func (s *Server) Report(w http.ResponseWriter, r *http.Request) {
	var req protocol.ReportRequest
	if err := decodeJSON(r, &req); err != nil || req.PID == "" || req.SN == "" || req.Ts == 0 || req.Payloads == nil {
		writeErr(w, protocol.CodeMalformedBody, "malformed request body")
		return
	}

	now := s.Now()
	ts := time.Unix(req.Ts, 0)
	if diff := now.Sub(ts); diff > TimeWindow || diff < -TimeWindow {
		writeErr(w, protocol.CodeTimestampOutOfWindow, "timestamp out of window")
		return
	}

	dev, _, err := s.Store.GetOrCreateDeviceBySN(req.PID, req.SN)
	if err != nil {
		writeErr(w, protocol.CodeServerError, "internal error")
		return
	}
	if dev.Status != model.DeviceStatusEnabled {
		writeErr(w, protocol.CodeDeviceDisabled, "device disabled")
		return
	}

	if err := s.Store.ProcessReport(dev, req.Payloads, now); err != nil {
		writeErr(w, protocol.CodeServerError, "internal error")
		return
	}

	writeJSON(w, 200, protocol.ReportResponse{C: protocol.CodeOK, T: now.Unix()})
}
