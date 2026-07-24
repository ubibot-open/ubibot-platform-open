// Package model holds the GORM row types persisted by the server. This is
// the durable book of record — device identity and telemetry history
// survive a restart because they live here instead of in an in-memory map.
package model

import "time"

// Device statuses. A disabled device is rejected by every device-facing
// endpoint (see internal/api) — this is the only lever an operator has
// over an existing device short of deleting it outright.
const (
	DeviceStatusEnabled  = 1
	DeviceStatusDisabled = 2
)

// Device is a device's identity plus the bookkeeping needed to show it in
// 设备管理 and judge whether it's online. Per docs
// UbiBot开放平台硬件通信协议.md, there is no provisioning step: a device
// shows up the moment it successfully calls POST /api/v1/data/report with
// an SN this platform hasn't seen before (see store.GetOrCreateDeviceBySN)
// — no secret, no signature, no activation handshake. PID/SN are the
// entire identity; Name is purely a display label an operator can set
// after the fact (see api's rename handler).
type Device struct {
	ID     uint   `gorm:"primaryKey"`
	PID    string `gorm:"size:64;not null"`
	SN     string `gorm:"size:64;not null;uniqueIndex"`
	Name   string `gorm:"size:128"`
	Status int    `gorm:"not null;default:1"`

	LastSeenAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Device) TableName() string { return "devices" }

// DeviceRecord is one persisted telemetry sample (protocol §4 payloads[]).
// Data is the JSON-encoded field1..field20 -> value map (see §5 of the
// doc); the unique index on (device_id, ts) is what implements "同一时间点
// 去重" — a duplicate insert is turned into a no-op (see store.SaveRecords)
// rather than erroring or double-counting.
type DeviceRecord struct {
	ID       uint  `gorm:"primaryKey"`
	DeviceID uint  `gorm:"not null;uniqueIndex:idx_device_ts"`
	Ts       int64 `gorm:"not null;uniqueIndex:idx_device_ts"`
	Data     string `gorm:"type:text;not null"`

	CreatedAt time.Time
}

func (DeviceRecord) TableName() string { return "device_records" }

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
// newly-saved telemetry record (see store.evaluateThresholdRules). Field is
// free text — there's no fixed enum, so it works unchanged against the new
// field1..field20 payload keys (see docs §5); an operator just types
// "field1" (or whatever custom field they're watching). Offline detection
// has no rule row — it's a structural check against Device.LastSeenAt run
// by the background sweep in cmd/server, not a user-configured condition.
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
	PermDeviceRead   = "device:read"
	PermDeviceWrite  = "device:write"
	PermAlertManage  = "alert:manage"
	PermSystemManage = "system:manage"
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

// AdminSession is an admin login session: an opaque bearer token in a
// table rather than a JWT — one session mechanism, one thing to reason
// about, no extra dependency.
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

// Notification levels/statuses/types (消息中心) — distinct from AlertEvent
// (device-condition specific, shown in 告警中心): a Notification covers
// system-level events more broadly.
const (
	NotificationLevelInfo     = "info"
	NotificationLevelWarning  = "warning"
	NotificationLevelCritical = "critical"

	NotificationStatusUnread = "unread"
	NotificationStatusRead   = "read"

	NotificationTypeAlert  = "alert"
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

// ApiKey authenticates third-party integrations against the read-only
// /api/open/v1 surface (see internal/api's RequireApiKey) — a separate
// credential from admin sessions, scoped narrower. Only KeyHash is
// persisted; the raw key is shown once at creation.
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

// FileAsset is a generic uploaded-file record (exports, attachments, etc).
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
// deploy. Type groups entries into a named dictionary; Key is the stored
// value, Label is what the UI shows for it.
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

// IconAsset is a custom SVG icon uploaded to override (or extend) the
// built-in field icon set the "数据仓库" (data warehouse) page renders. Key
// is the field it applies to -- "field1"/"field2"/"field3" by the doc's
// default convention, or any custom field1..field20 name a deployment
// chooses to give a distinct icon; at most one icon per key -- uploading
// again for the same key replaces it (see store.UpsertIcon).
type IconAsset struct {
	ID   uint   `gorm:"primaryKey"`
	Key  string `gorm:"size:64;not null;uniqueIndex"`
	Name string `gorm:"size:128;not null"`
	SVG  string `gorm:"type:text;not null"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (IconAsset) TableName() string { return "icon_assets" }
