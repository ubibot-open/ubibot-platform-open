// Package protocol defines the wire format shared by the device-facing HTTP
// endpoints, mirroring docs/UbiBot开放平台硬件通信协议.docx.
package protocol

// Business status codes (the "c" field). Zero means success; non-zero codes
// map to the HTTP status shown in the doc's error table.
const (
	CodeOK             = 0
	CodeSignMismatch   = 1002 // bad signature, invalid/used nonce, or timestamp out of window
	CodeMalformedBody  = 1003
	CodeTokenInvalid   = 1101
	CodeTokenExpired   = 1102
	CodeDeviceNotFound = 1103
	CodeRateLimited    = 1900
	CodeServerError    = 5000
)

// TimeSyncRequest is POST /api/v1/auth/time. It carries no timestamp: the
// endpoint validates device identity only, so devices with no local clock
// reference (first boot, RTC lost) can still call it.
type TimeSyncRequest struct {
	PID  string `json:"pid" binding:"required"`
	SN   string `json:"sn" binding:"required"`
	Sign string `json:"sign" binding:"required"`
}

// TimeSyncResponse returns the server's current time plus a single-use nonce
// for the following activation request.
type TimeSyncResponse struct {
	C int    `json:"c"`
	T int64  `json:"t"`
	N string `json:"n"`
}

// ActivateRequest is POST /api/v1/auth/activate. N is optional: present when
// the device just called the time-sync endpoint (nonce-bound, replay-proof
// even without a trustworthy timestamp); absent when the device already has
// a synced clock, in which case Ts is checked against a time window instead.
type ActivateRequest struct {
	PID  string `json:"pid" binding:"required"`
	SN   string `json:"sn" binding:"required"`
	Ts   int64  `json:"ts" binding:"required"`
	N    string `json:"n"`
	Sign string `json:"sign" binding:"required"`
}

// ActivateResponse carries the session token on success.
type ActivateResponse struct {
	C     int    `json:"c"`
	Token string `json:"token"`
	Exp   int64  `json:"exp"`
}

// Record is one sampled time point; D maps sensor name to value (scalar or,
// for compound sensors such as NPK, a nested object).
type Record struct {
	Ts int64                  `json:"ts" binding:"required"`
	D  map[string]interface{} `json:"d" binding:"required"`
}

// ReportRequest is POST /api/v1/data/report. Recs supports batching multiple
// time points (e.g. buffered offline data) in a single upload. Ack confirms
// commands the device has already executed (see CmdItem).
type ReportRequest struct {
	DID  string   `json:"did" binding:"required"`
	Recs []Record `json:"recs" binding:"required"`
	Ack  []string `json:"ack"`
}

// Config is the device's sampling/upload configuration.
type Config struct {
	CI int      `json:"ci"`
	UI int      `json:"ui"`
	FE []string `json:"fe,omitempty"`
}

// CmdItem is one queued control instruction delivered piggybacked on a
// report response. The device must echo Id back in a later ReportRequest.Ack
// once executed.
type CmdItem struct {
	ID string                 `json:"id"`
	Tp string                 `json:"tp"`
	A  map[string]interface{} `json:"a,omitempty"`
}

// ReportResponse is the reply to a data upload. Cfg is only populated when
// the device's configuration changed since it was last delivered; Cmd is
// only populated when commands are queued — both omitted otherwise to keep
// the body small.
type ReportResponse struct {
	C   int        `json:"c"`
	T   int64      `json:"t"`
	Cfg *Config    `json:"cfg,omitempty"`
	Cmd []CmdItem  `json:"cmd,omitempty"`
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
	case CodeSignMismatch, CodeMalformedBody:
		return 400
	case CodeTokenInvalid, CodeTokenExpired, CodeDeviceNotFound:
		return 401
	case CodeRateLimited:
		return 429
	default:
		return 500
	}
}
