// Package protocol defines the wire format shared by the device-facing
// HTTP endpoints, mirroring docs/UbiBot开放平台硬件通信协议.md — deliberately
// tiny: no signing, no session tokens, no command channel. A device only
// ever needs pid+sn to identify itself.
package protocol

// Business status codes (the "c" field). Zero means success; non-zero
// codes map to the HTTP status shown in the doc's error table (§7).
const (
	CodeOK                   = 0
	CodeTimestampOutOfWindow = 1002 // ts outside the ±5 minute window
	CodeMalformedBody        = 1003
	CodeDeviceDisabled       = 1103 // device exists but was disabled by an operator
	CodeRateLimited          = 1900
	CodeServerError          = 5000
)

// TimeSyncRequest is POST /api/v1/auth/time (docs §3) — no signature, no
// timestamp: a device with no clock reference yet can call this purely to
// learn the current time.
type TimeSyncRequest struct {
	PID string `json:"pid" binding:"required"`
	SN  string `json:"sn" binding:"required"`
}

// TimeSyncResponse returns the server's current time.
type TimeSyncResponse struct {
	C int   `json:"c"`
	T int64 `json:"t"`
}

// Payload is one sampled time point (docs §4); Feed maps field1..field20
// to numeric values. There's no fixed sensor vocabulary — the platform
// just stores whatever keys arrive (see docs §5 for the field1/2/3
// default-meaning convention, which is a display-only convention, not
// something this layer enforces).
type Payload struct {
	Ts   int64              `json:"ts" binding:"required"`
	Feed map[string]float64 `json:"feed" binding:"required"`
}

// ReportRequest is POST /api/v1/data/report (docs §4) — the device's only
// other endpoint besides time-sync. Identity is just PID+SN, in the body,
// unauthenticated; Ts is the request's own timestamp (checked against a
// ±5 minute window), separate from each Payload's own Ts (which may be
// older, for batched offline-buffered samples).
type ReportRequest struct {
	PID      string    `json:"pid" binding:"required"`
	SN       string    `json:"sn" binding:"required"`
	Ts       int64     `json:"ts" binding:"required"`
	Payloads []Payload `json:"payloads" binding:"required"`
}

// ReportResponse is the reply to a data upload — deliberately minimal,
// just an ack and the server's clock for reference.
type ReportResponse struct {
	C int   `json:"c"`
	T int64 `json:"t"`
}

// ErrorResponse is the generic error envelope for every endpoint.
type ErrorResponse struct {
	C int    `json:"c"`
	M string `json:"m"`
}

// HTTPStatusFor maps a business code to the HTTP status the doc's error
// table specifies.
func HTTPStatusFor(code int) int {
	switch code {
	case CodeOK:
		return 200
	case CodeTimestampOutOfWindow, CodeMalformedBody:
		return 400
	case CodeDeviceDisabled:
		return 401
	case CodeRateLimited:
		return 429
	default:
		return 500
	}
}
