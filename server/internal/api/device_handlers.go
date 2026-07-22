package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/auth"
	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/protocol"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, protocol.HTTPStatusFor(code), protocol.ErrorResponse{C: code, M: msg})
}

// deviceLookupFailed replies with the same error for "no such device",
// "device disabled", and "signature doesn't match" so a caller can't use
// this endpoint to probe which serial numbers are registered or which
// ones an operator has disabled.
func deviceLookupFailed(w http.ResponseWriter) {
	writeErr(w, protocol.CodeSignMismatch, "sign mismatch")
}

// lookupActiveDevice fetches a device by SN and confirms both its pid and
// its enabled status, folding "not found", "pid mismatch", and "disabled"
// (which, from this endpoint's point of view, also covers Pending —
// see model.DeviceStatusPending) into a single outcome every caller in
// this file needs to check.
func (s *Server) lookupActiveDevice(sn, pid string) (*model.Device, bool) {
	dev, err := s.Store.DeviceBySN(sn)
	if err != nil {
		return nil, false
	}
	if dev.PID != pid || dev.Status != model.DeviceStatusEnabled {
		return nil, false
	}
	return dev, true
}

// maybeAutoRegisterDevice creates a Pending, self-registered device row
// the first time a completely unrecognized SN attempts to activate — see
// store.AutoRegisterDevice for why this can't (and doesn't try to)
// authenticate the caller, and api.ApproveDevice/SetDeviceStatus for how
// an operator later approves or rejects it from 设备管理. Existing SNs
// (whether enabled, disabled, or already pending) are left alone — this
// only fires on true first contact, and only from Activate: TimeSync is
// just a clock-sync convenience step with nothing to approve, so an
// unrecognized SN there is simply rejected as always.
func (s *Server) maybeAutoRegisterDevice(pid, sn string) {
	if _, err := s.Store.DeviceBySN(sn); !errors.Is(err, store.ErrNotFound) {
		return
	}
	secret, err := auth.NewDeviceSecret()
	if err != nil {
		return
	}
	// Errors here (most likely a race with a concurrent duplicate
	// first-contact request tripping the SN unique index) are swallowed:
	// either way the caller falls through to the same generic rejection
	// below, and if a row now exists, that's all this was trying to do.
	_, _ = s.Store.AutoRegisterDevice(pid, sn, secret)
}

// TimeSync handles POST /api/v1/auth/time. It validates the device's
// identity (via a signature over pid+sn) but deliberately does not check
// any timestamp — that's the point: a device with no clock reference yet
// can still call this to learn the current time.
func (s *Server) TimeSync(w http.ResponseWriter, r *http.Request) {
	var req protocol.TimeSyncRequest
	if err := decodeJSON(r, &req); err != nil || req.PID == "" || req.SN == "" || req.Sign == "" {
		writeErr(w, protocol.CodeMalformedBody, "malformed request body")
		return
	}

	dev, ok := s.lookupActiveDevice(req.SN, req.PID)
	if !ok || !auth.Verify(dev.Secret, req.Sign, req.PID, req.SN) {
		deviceLookupFailed(w)
		return
	}

	now := s.Now()
	nonce, err := s.Nonces.Issue(req.SN, now)
	if err != nil {
		writeErr(w, protocol.CodeServerError, "internal error")
		return
	}

	writeJSON(w, 200, protocol.TimeSyncResponse{C: protocol.CodeOK, T: now.Unix(), N: nonce})
}

// Activate handles POST /api/v1/auth/activate. Two authentication paths:
//   - N present: the device just called TimeSync and is using the
//     returned (t, n) pair. The nonce is single-use, so replaying this
//     exact request cannot activate twice — no trustworthy timestamp is
//     required.
//   - N absent: the device already has a synced clock and signs with its
//     own ts, checked against a ±5 minute window AND a per-device
//     monotonic floor (store.CheckAndAdvanceActivateTS) — the window
//     alone would let a captured request be replayed repeatedly within
//     those 5 minutes; requiring ts to strictly advance closes that gap.
//
// Both paths share one signing formula, sign = HMAC(secret, pid+sn+ts+n),
// with n treated as the empty string when the field is omitted.
//
// A third case this handles: an SN the platform has never seen before.
// There's no stored secret to check such a request's signature against,
// so it's never going to succeed here regardless — but before replying
// with the usual generic rejection, maybeAutoRegisterDevice records the
// attempt as a Pending, self-registered device (model.DeviceSourceSelfRegistered)
// so it shows up in 设备管理 for an operator to review. The device itself
// sees no difference from any other rejected attempt; it's expected to
// keep retrying until an operator approves it (api.ApproveDevice), at
// which point the normal signature check above starts applying (once the
// operator has flashed the auto-generated secret into it).
func (s *Server) Activate(w http.ResponseWriter, r *http.Request) {
	var req protocol.ActivateRequest
	if err := decodeJSON(r, &req); err != nil || req.PID == "" || req.SN == "" || req.Sign == "" || req.Ts == 0 {
		writeErr(w, protocol.CodeMalformedBody, "malformed request body")
		return
	}

	dev, ok := s.lookupActiveDevice(req.SN, req.PID)
	if !ok {
		s.maybeAutoRegisterDevice(req.PID, req.SN)
		deviceLookupFailed(w)
		return
	}
	if !auth.Verify(dev.Secret, req.Sign, req.PID, req.SN, auth.FormatTs(req.Ts), req.N) {
		deviceLookupFailed(w)
		return
	}

	now := s.Now()
	if req.N != "" {
		if !s.Nonces.Consume(req.SN, req.N, now) {
			writeErr(w, protocol.CodeSignMismatch, "nonce invalid or expired")
			return
		}
	} else {
		ts := time.Unix(req.Ts, 0)
		if diff := now.Sub(ts); diff > auth.TimeWindow || diff < -auth.TimeWindow {
			writeErr(w, protocol.CodeSignMismatch, "timestamp out of window")
			return
		}
		advanced, err := s.Store.CheckAndAdvanceActivateTS(dev.ID, req.Ts)
		if err != nil {
			writeErr(w, protocol.CodeServerError, "internal error")
			return
		}
		if !advanced {
			writeErr(w, protocol.CodeSignMismatch, "timestamp not advancing")
			return
		}
	}

	token, ttl, err := s.Store.IssueDeviceToken(dev.ID)
	if err != nil {
		writeErr(w, protocol.CodeServerError, "internal error")
		return
	}
	if err := s.Store.MarkDeviceActivated(dev.ID); err != nil {
		writeErr(w, protocol.CodeServerError, "internal error")
		return
	}

	writeJSON(w, 200, protocol.ActivateResponse{
		C:     protocol.CodeOK,
		Token: token,
		Exp:   int64(ttl.Seconds()),
	})
}

// Report handles POST /api/v1/data/report: token-authenticated telemetry
// upload, with device configuration and queued commands piggybacked on
// the response since the device cannot receive an unsolicited push.
func (s *Server) Report(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-IoT-Token")
	if token == "" {
		writeErr(w, protocol.CodeTokenInvalid, "missing token")
		return
	}

	dev, remaining, status, err := s.Store.ValidateDeviceToken(token)
	if err != nil {
		writeErr(w, protocol.CodeServerError, "internal error")
		return
	}
	switch status {
	case store.DeviceTokenNotFound:
		writeErr(w, protocol.CodeTokenInvalid, "token invalid")
		return
	case store.DeviceTokenExpired:
		writeErr(w, protocol.CodeTokenExpired, "token expired")
		return
	}
	if dev.Status != model.DeviceStatusEnabled {
		// A disabled device is treated the same as "device not found" to
		// every device-facing endpoint (see lookupActiveDevice) — a still-
		// valid token from before it was disabled shouldn't be able to
		// keep uploading data.
		writeErr(w, protocol.CodeDeviceNotFound, "device disabled")
		return
	}

	var req protocol.ReportRequest
	if err := decodeJSON(r, &req); err != nil || req.DID == "" || req.Recs == nil {
		writeErr(w, protocol.CodeMalformedBody, "malformed request body")
		return
	}

	if req.DID != dev.SN {
		writeErr(w, protocol.CodeDeviceNotFound, "device does not match token")
		return
	}

	outcome, err := s.Store.ProcessReport(dev, req.Recs, req.Ack, req.Nak, req.Ota, s.Now())
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, protocol.CodeDeviceNotFound, "device not found")
		return
	}
	if err != nil {
		writeErr(w, protocol.CodeServerError, "internal error")
		return
	}

	w.Header().Set("X-Token-Expires-In", strconv.Itoa(int(remaining.Seconds())))
	writeJSON(w, 200, protocol.ReportResponse{
		C:   protocol.CodeOK,
		T:   s.Now().Unix(),
		Cfg: outcome.Cfg,
		Cmd: outcome.Cmd,
	})
}

// Poll handles GET /api/v1/device/poll (protocol §7.1): the same cfg/cmd
// payload as Report, without requiring a telemetry batch — for devices
// that want to check for freshly-dispatched commands more often than
// their upload interval without generating a data-report history entry.
func (s *Server) Poll(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-IoT-Token")
	if token == "" {
		writeErr(w, protocol.CodeTokenInvalid, "missing token")
		return
	}

	dev, remaining, status, err := s.Store.ValidateDeviceToken(token)
	if err != nil {
		writeErr(w, protocol.CodeServerError, "internal error")
		return
	}
	switch status {
	case store.DeviceTokenNotFound:
		writeErr(w, protocol.CodeTokenInvalid, "token invalid")
		return
	case store.DeviceTokenExpired:
		writeErr(w, protocol.CodeTokenExpired, "token expired")
		return
	}
	if dev.Status != model.DeviceStatusEnabled {
		writeErr(w, protocol.CodeDeviceNotFound, "device disabled")
		return
	}

	if did := r.URL.Query().Get("did"); did != "" && did != dev.SN {
		writeErr(w, protocol.CodeDeviceNotFound, "device does not match token")
		return
	}

	outcome, err := s.Store.ProcessReport(dev, nil, nil, nil, nil, s.Now())
	if err != nil {
		writeErr(w, protocol.CodeServerError, "internal error")
		return
	}

	w.Header().Set("X-Token-Expires-In", strconv.Itoa(int(remaining.Seconds())))
	writeJSON(w, 200, protocol.ReportResponse{
		C:   protocol.CodeOK,
		T:   s.Now().Unix(),
		Cfg: outcome.Cfg,
		Cmd: outcome.Cmd,
	})
}
