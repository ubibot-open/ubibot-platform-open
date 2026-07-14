package api

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ubibot/ubibot-platform-open/internal/auth"
	"github.com/ubibot/ubibot-platform-open/internal/protocol"
)

func writeErr(c *gin.Context, code int, msg string) {
	c.JSON(protocol.HTTPStatusFor(code), protocol.ErrorResponse{C: code, M: msg})
}

// deviceLookupFailed replies with the same error for "no such device" and
// "signature doesn't match" so a caller can't use this endpoint to probe
// which serial numbers are registered.
func deviceLookupFailed(c *gin.Context) {
	writeErr(c, protocol.CodeSignMismatch, "sign mismatch")
}

// TimeSync handles POST /api/v1/auth/time. It validates the device's
// identity (via a signature over pid+sn) but deliberately does not check
// any timestamp — that's the point: a device with no clock reference yet
// can still call this to learn the current time.
func (s *Server) TimeSync(c *gin.Context) {
	var req protocol.TimeSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErr(c, protocol.CodeMalformedBody, "malformed request body")
		return
	}

	dev, ok := s.Store.Device(req.SN)
	if !ok || dev.PID != req.PID {
		deviceLookupFailed(c)
		return
	}
	if !auth.Verify(dev.Secret, req.Sign, req.PID, req.SN) {
		deviceLookupFailed(c)
		return
	}

	now := s.Now()
	nonce, err := s.Nonces.Issue(req.SN, now)
	if err != nil {
		writeErr(c, protocol.CodeServerError, "internal error")
		return
	}

	c.JSON(200, protocol.TimeSyncResponse{C: protocol.CodeOK, T: now.Unix(), N: nonce})
}

// Activate handles POST /api/v1/auth/activate. Two authentication paths:
//   - N present: the device just called TimeSync and is using the returned
//     (t, n) pair. The nonce is single-use, so replaying this exact request
//     cannot activate twice — no trustworthy timestamp is required.
//   - N absent: the device already has a synced clock and signs with its
//     own ts, checked against a ±5 minute window (the original scheme).
//
// Both paths share one signing formula, sign = HMAC(secret, pid+sn+ts+n),
// with n treated as the empty string when the field is omitted.
func (s *Server) Activate(c *gin.Context) {
	var req protocol.ActivateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErr(c, protocol.CodeMalformedBody, "malformed request body")
		return
	}

	dev, ok := s.Store.Device(req.SN)
	if !ok || dev.PID != req.PID {
		deviceLookupFailed(c)
		return
	}

	if !auth.Verify(dev.Secret, req.Sign, req.PID, req.SN, auth.FormatTs(req.Ts), req.N) {
		deviceLookupFailed(c)
		return
	}

	now := s.Now()
	if req.N != "" {
		if !s.Nonces.Consume(req.SN, req.N, now) {
			writeErr(c, protocol.CodeSignMismatch, "nonce invalid or expired")
			return
		}
	} else {
		ts := time.Unix(req.Ts, 0)
		if diff := now.Sub(ts); diff > auth.TimeWindow || diff < -auth.TimeWindow {
			writeErr(c, protocol.CodeSignMismatch, "timestamp out of window")
			return
		}
	}

	token, ttl, err := s.Tokens.Issue(req.SN, now)
	if err != nil {
		writeErr(c, protocol.CodeServerError, "internal error")
		return
	}

	c.JSON(200, protocol.ActivateResponse{
		C:     protocol.CodeOK,
		Token: token,
		Exp:   int64(ttl.Seconds()),
	})
}

// Report handles POST /api/v1/data/report: token-authenticated telemetry
// upload, with device configuration and queued commands piggybacked on the
// response since the device cannot receive an unsolicited push.
func (s *Server) Report(c *gin.Context) {
	token := c.GetHeader("X-IoT-Token")
	if token == "" {
		writeErr(c, protocol.CodeTokenInvalid, "missing token")
		return
	}

	now := s.Now()
	did, remaining, status := s.Tokens.Validate(token, now)
	switch status {
	case auth.TokenNotFound:
		writeErr(c, protocol.CodeTokenInvalid, "token invalid")
		return
	case auth.TokenExpired:
		writeErr(c, protocol.CodeTokenExpired, "token expired")
		return
	}

	var req protocol.ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErr(c, protocol.CodeMalformedBody, "malformed request body")
		return
	}

	if req.DID != did {
		writeErr(c, protocol.CodeDeviceNotFound, "device does not match token")
		return
	}

	outcome := s.Store.ProcessReport(did, req.Recs, req.Ack)

	c.Header("X-Token-Expires-In", strconv.Itoa(int(remaining.Seconds())))
	c.JSON(200, protocol.ReportResponse{
		C:   protocol.CodeOK,
		T:   now.Unix(),
		Cfg: outcome.Cfg,
		Cmd: outcome.Cmd,
	})
}
