// Package model holds the GORM row types persisted by the server. This is
// the durable book of record — device identity, issued tokens, telemetry
// history, and the command queue all survive a restart because they live
// here instead of in an in-memory map.
package model

import "time"

// Device statuses. Disabled behaves like "unknown device" to every
// device-facing endpoint (see internal/api), so disabling one doesn't leak
// which serials exist.
const (
	DeviceStatusEnabled  = 1
	DeviceStatusDisabled = 2
)

// Device is the factory-provisioned identity triple plus the
// sampling/upload configuration the server pushes down. Did (used in
// /api/v1/data/report) is the same value as SN — the protocol doc has no
// separate device-registration step that would mint a distinct id.
type Device struct {
	ID     uint   `gorm:"primaryKey"`
	PID    string `gorm:"size:64;not null"`
	SN     string `gorm:"size:64;not null;uniqueIndex"`
	Secret string `gorm:"size:128;not null"`
	Name   string `gorm:"size:128"`
	Status int    `gorm:"not null;default:1"`

	// Config pushed to the device (protocol §7 cfg block). FE is stored as
	// a JSON array string; empty means "all sensors enabled".
	CI int    `gorm:"not null;default:30"`
	UI int    `gorm:"not null;default:600"`
	FE string `gorm:"type:text"`

	// CfgVersion bumps on every config change; LastSentCfgVersion is the
	// version last delivered to the device. The two only match once the
	// device has picked up the latest config — that's what makes cfg
	// diff-only (protocol §7: "仅当配置变化时才返回").
	CfgVersion         int
	LastSentCfgVersion int

	// LastActivateTS guards the fallback activation path (no nonce, ±5min
	// window): only a strictly increasing ts is accepted, which closes the
	// replay gap a bare time window leaves open (see docs §4 note).
	LastActivateTS int64

	LastSeenAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Device) TableName() string { return "devices" }

// DeviceToken is an issued session token (protocol §4 "设备激活"). Persisting
// this — instead of the in-memory map the reference server used — is what
// lets a device's session survive a server restart.
type DeviceToken struct {
	Token     string `gorm:"primaryKey;size:64"`
	DeviceID  uint   `gorm:"not null;index"`
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (DeviceToken) TableName() string { return "device_tokens" }

// DeviceRecord is one persisted telemetry sample (protocol §5 recs[]). The
// unique index on (device_id, ts) is what implements "同一(did,ts)去重" —
// a duplicate insert is turned into a no-op (see store.SaveRecords) rather
// than erroring or double-counting.
type DeviceRecord struct {
	ID       uint `gorm:"primaryKey"`
	DeviceID uint `gorm:"not null;uniqueIndex:idx_device_ts"`
	Ts       int64 `gorm:"not null;uniqueIndex:idx_device_ts"`
	Data     string `gorm:"type:text;not null"`

	CreatedAt time.Time
}

func (DeviceRecord) TableName() string { return "device_records" }

// Command lifecycle: Pending until the device acks or naks it.
const (
	CommandStatusPending = "pending"
	CommandStatusAcked   = "acked"
	CommandStatusNacked  = "nacked"
)

// DeviceCommand is one queued control instruction (protocol §7 cmd[]).
// CmdID is the short id ("c123") the wire protocol uses for ack/nak;
// keeping the row after it's acked (instead of deleting it, like the
// in-memory reference server did) is what lets the admin UI show history.
type DeviceCommand struct {
	ID         uint   `gorm:"primaryKey"`
	CmdID      string `gorm:"size:24;not null;uniqueIndex"`
	DeviceID   uint   `gorm:"not null;index"`
	Type       string `gorm:"size:24;not null"`
	Args       string `gorm:"type:text"`
	Status     string `gorm:"size:16;not null;default:pending"`
	NakMessage string `gorm:"size:255"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (DeviceCommand) TableName() string { return "device_commands" }

// Probe lifecycle: Pending until the set_probe command that (up)serted or
// removed it is acked/nacked (see store.applyProbeCommandOutcome).
const (
	ProbeStatusPending  = "pending"
	ProbeStatusApplied  = "applied"
	ProbeStatusFailed   = "failed"
	ProbeStatusRemoving = "removing"
)

// DeviceProbe is a custom sensor read configuration (protocol §7.2
// set_probe) — RS485/Modbus register maps, analog scaling, etc. that can't
// be baked into firmware because they vary by whatever's physically wired
// to the device. Params holds the protocol-specific fields (addr/fc/reg/
// cnt/dtype/byte_order/scale/offset/ci/timeout/retry) as a JSON blob
// rather than one column each — the field set differs by iface/proto, and
// exploding every combination into sparse columns wouldn't read any
// clearer than the JSON the wire protocol already uses.
type DeviceProbe struct {
	ID       uint   `gorm:"primaryKey"`
	DeviceID uint   `gorm:"not null;uniqueIndex:idx_device_pid"`
	Pid      string `gorm:"size:32;not null;uniqueIndex:idx_device_pid"`
	Key      string `gorm:"size:64;not null"`
	Iface    string `gorm:"size:32;not null"`
	Proto    string `gorm:"size:32;not null"`
	Params   string `gorm:"type:text"`

	Status        string `gorm:"size:16;not null;default:pending"`
	LastCommandID string `gorm:"size:24"`
	LastError     string `gorm:"size:255"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (DeviceProbe) TableName() string { return "device_probes" }

// Alert types and statuses.
const (
	AlertTypeThreshold = "threshold"
	AlertTypeOffline   = "offline"

	AlertStatusOpen     = "open"
	AlertStatusResolved = "resolved"
)

// Comparison operators an AlertRule can use against a telemetry field.
const (
	AlertOpGT = ">"
	AlertOpGE = ">="
	AlertOpLT = "<"
	AlertOpLE = "<="
	AlertOpEQ = "=="
)

// AlertRule is a per-device threshold check, evaluated against every
// newly-saved telemetry record (see store.evaluateThresholdRules). Offline
// detection has no rule row — it's a structural check against
// Device.LastSeenAt run by the background sweep in cmd/server, not a
// user-configured condition.
type AlertRule struct {
	ID        uint    `gorm:"primaryKey"`
	DeviceID  uint    `gorm:"not null;index"`
	Field     string  `gorm:"size:64;not null"`
	Op        string  `gorm:"size:4;not null"`
	Threshold float64 `gorm:"not null"`
	Enabled   bool    `gorm:"not null;default:true"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (AlertRule) TableName() string { return "alert_rules" }

// AlertEvent is one open-or-resolved occurrence of a rule firing (or a
// device going offline). RuleID is 0 for offline events. Re-triggering an
// already-open event for the same rule/device is a no-op (see
// store.evaluateThresholdRules) so a flapping sensor doesn't spam the
// alert center with duplicate open rows.
type AlertEvent struct {
	ID       uint   `gorm:"primaryKey"`
	DeviceID uint   `gorm:"not null;index"`
	RuleID   uint   `gorm:"index"`
	Type     string `gorm:"size:16;not null"`
	Message  string `gorm:"size:255;not null"`
	Status   string `gorm:"size:16;not null;default:open;index"`

	TriggeredAt time.Time
	ResolvedAt  *time.Time
}

func (AlertEvent) TableName() string { return "alert_events" }

// Built-in role codes. RoleSuper is seeded once at bootstrap (see
// cmd/server/main.go) and always passes permission checks regardless of
// its stored Permissions — a fixed escape hatch so a botched permissions
// edit can never lock every admin out.
const RoleSuper = "super_admin"

// Permission codes checked by internal/api's RequirePermission. Keeping
// this as a flat list of strings (stored space-separated on Role, see
// Permissions) instead of a normalized join table is a deliberate P1
// simplification — revisit if the permission set outgrows "one row per
// role, one column of codes".
const (
	PermDeviceRead     = "device:read"
	PermDeviceWrite    = "device:write"
	PermCommandWrite   = "command:write"
	PermAlertManage    = "alert:manage"
	PermSystemManage   = "system:manage"
)

// Role groups a set of permission codes under a name an AdminUser can be
// assigned to.
type Role struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"size:64;not null"`
	Code        string `gorm:"size:64;not null;uniqueIndex"`
	Permissions string `gorm:"type:text"` // space-separated permission codes, or "*" for all

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Role) TableName() string { return "roles" }

// AdminUser is a back-office operator account. There is no separate
// customer/tenant account type yet — out of scope for this slice.
type AdminUser struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"size:64;not null;uniqueIndex"`
	PasswordHash string `gorm:"size:128;not null"`
	RoleID       uint   `gorm:"not null;index"`

	CreatedAt time.Time
}

func (AdminUser) TableName() string { return "admin_users" }

// AdminSession is an admin login session, deliberately the same
// opaque-bearer-token-in-a-table shape as DeviceToken rather than JWT —
// one session mechanism, one thing to reason about, no extra dependency.
type AdminSession struct {
	Token     string `gorm:"primaryKey;size:64"`
	AdminID   uint   `gorm:"not null;index"`
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (AdminSession) TableName() string { return "admin_sessions" }

// AuditLog records a mutating admin action for after-the-fact review.
// Written by internal/api's writeAudit helper at the point each mutating
// handler succeeds — read-only endpoints (list/get) are not audited.
type AuditLog struct {
	ID         uint   `gorm:"primaryKey"`
	AdminID    uint   `gorm:"not null;index"`
	Username   string `gorm:"size:64;not null"`
	Action     string `gorm:"size:64;not null"`
	TargetType string `gorm:"size:32"`
	TargetID   uint
	Detail     string `gorm:"type:text"`
	IP         string `gorm:"size:64"`

	CreatedAt time.Time
}

func (AuditLog) TableName() string { return "audit_logs" }
