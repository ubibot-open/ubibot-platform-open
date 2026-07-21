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

	// Activated records whether this device has ever completed the
	// activation handshake (protocol §4) at least once — separate from
	// Status (enable/disable is an operator decision; Activated is a fact
	// about the device's own history that, once true, never reverts).
	// A device that has never activated has never been online, so the
	// offline-alert sweep (see store.OfflineSweep) skips it rather than
	// immediately raising an alert for a device that was never up.
	Activated bool `gorm:"not null;default:false"`

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

// Firmware is an uploaded OTA image (protocol §7.3) plus the integrity
// metadata devices need to safely apply it. The binary itself lives on
// disk (Path) — sqlite is fine for metadata, not for multi-MB blobs.
type Firmware struct {
	ID        uint   `gorm:"primaryKey"`
	PID       string `gorm:"size:64;not null;index"` // which product this firmware targets
	Version   string `gorm:"size:32;not null"`
	Filename  string `gorm:"size:255;not null"`
	Path      string `gorm:"size:255;not null"`
	Size      int64  `gorm:"not null"`
	SHA256    string `gorm:"size:64;not null"`
	Signature string `gorm:"size:512"` // optional, protocol §7.3 a.sig

	CreatedAt time.Time
}

func (Firmware) TableName() string { return "firmwares" }

// OTA task states (protocol §7.3's ota.state). Pending is this server's
// own bookkeeping value for "dispatched, no progress reported yet" — it
// never appears on the wire, the device only ever reports the states
// from downloading onward.
const (
	OtaStatePending     = "pending"
	OtaStateDownloading = "downloading"
	OtaStateVerifying   = "verifying"
	OtaStateFlashing    = "flashing"
	OtaStateRebooting   = "rebooting"
	OtaStateSuccess     = "success"
	OtaStateFailed      = "failed"
	OtaStateRolledBack  = "rolled_back"
)

// DeviceOTA tracks the single in-flight (or most recently finished) OTA
// task for a device — one row per device, overwritten by each new
// dispatch, since only one upgrade is ever in flight at a time.
type DeviceOTA struct {
	ID         uint   `gorm:"primaryKey"`
	DeviceID   uint   `gorm:"not null;uniqueIndex"`
	FirmwareID uint   `gorm:"not null"`
	CmdID      string `gorm:"size:24;not null"`
	Version    string `gorm:"size:32;not null"`
	State      string `gorm:"size:16;not null"`
	Progress   int
	LastError  string `gorm:"size:255"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (DeviceOTA) TableName() string { return "device_otas" }

// Notification levels/statuses/types (消息中心) — distinct from AlertEvent
// (device-condition specific, shown in 告警中心): a Notification also
// covers things like OTA outcomes and other system-level events.
const (
	NotificationLevelInfo     = "info"
	NotificationLevelWarning  = "warning"
	NotificationLevelCritical = "critical"

	NotificationStatusUnread = "unread"
	NotificationStatusRead   = "read"

	NotificationTypeAlert  = "alert"
	NotificationTypeOta    = "ota"
	NotificationTypeSystem = "system"
)

// Notification is a system-generated message surfaced in the admin
// header bell.
type Notification struct {
	ID      uint   `gorm:"primaryKey"`
	Type    string `gorm:"size:32;not null"`
	Level   string `gorm:"size:16;not null;default:info"`
	Title   string `gorm:"size:128;not null"`
	Content string `gorm:"type:text"`
	Status  string `gorm:"size:16;not null;default:unread;index"`

	CreatedAt time.Time
}

func (Notification) TableName() string { return "notifications" }

// Scheduled-task schedule kinds — kept to these two common cases instead
// of a full cron expression parser (which would also mean a new module
// dependency this sandbox's restricted network can't necessarily fetch).
const (
	ScheduleTypeInterval = "interval"
	ScheduleTypeDaily    = "daily"
)

// ScheduledTask periodically (or once, if disabled after the first run is
// desired) queues a command for a device — e.g. a nightly reboot — without
// an operator manually dispatching it each time. DeviceID 0 means "every
// enabled device".
type ScheduledTask struct {
	ID       uint   `gorm:"primaryKey"`
	Name     string `gorm:"size:128;not null"`
	DeviceID uint   `gorm:"index"`
	CmdType  string `gorm:"size:24;not null"`
	CmdArgs  string `gorm:"type:text"`

	ScheduleType    string `gorm:"size:16;not null"`
	IntervalSeconds int
	DailyAtMinute   int // minutes since local midnight, for daily schedules

	Enabled   bool `gorm:"not null;default:true"`
	NextRunAt time.Time
	LastRunAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (ScheduledTask) TableName() string { return "scheduled_tasks" }

// ApiKey authenticates third-party integrations against the read-only
// /api/open/v1 surface (see internal/api's RequireApiKey) — a separate
// credential from admin sessions and device tokens, scoped narrower than
// either. Only KeyHash is persisted; the raw key is shown once at
// creation, the same pattern as a device secret.
type ApiKey struct {
	ID         uint   `gorm:"primaryKey"`
	Name       string `gorm:"size:128;not null"`
	KeyHash    string `gorm:"size:128;not null;uniqueIndex"`
	Prefix     string `gorm:"size:12;not null"` // shown in the UI so operators can tell keys apart without re-revealing them
	Revoked    bool   `gorm:"not null;default:false"`
	LastUsedAt *time.Time

	CreatedAt time.Time
}

func (ApiKey) TableName() string { return "api_keys" }

// FileAsset is a generic uploaded-file record (exports, attachments,
// etc.) — separate from Firmware, which has its own OTA-specific
// integrity columns and lifecycle.
type FileAsset struct {
	ID       uint   `gorm:"primaryKey"`
	Category string `gorm:"size:32;not null;index"`
	Filename string `gorm:"size:255;not null"`
	Path     string `gorm:"size:255;not null"`
	Size     int64  `gorm:"not null"`
	SHA256   string `gorm:"size:64"`

	CreatedAt time.Time
}

func (FileAsset) TableName() string { return "file_assets" }

// DictEntry is a small key/value enumeration operators can edit without a
// deploy — e.g. display labels for command types. Type groups entries
// into a named dictionary (e.g. "command_type"); Key is the stored value,
// Label is what the UI shows for it.
type DictEntry struct {
	ID    uint   `gorm:"primaryKey"`
	Type  string `gorm:"size:64;not null;index"`
	Key   string `gorm:"size:64;not null"`
	Label string `gorm:"size:128;not null"`
	Sort  int

	CreatedAt time.Time
}

func (DictEntry) TableName() string { return "dict_entries" }

// SystemParam is a small whitelist of runtime-tunable settings. Not every
// key changes behavior — see internal/api/param_handlers.go and main.go
// for which ones are actually read back into live server state versus
// purely informational.
type SystemParam struct {
	Key         string `gorm:"primaryKey;size:64"`
	Value       string `gorm:"size:255;not null"`
	Description string `gorm:"size:255"`

	UpdatedAt time.Time
}

func (SystemParam) TableName() string { return "system_params" }
