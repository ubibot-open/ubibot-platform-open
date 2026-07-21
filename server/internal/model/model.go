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

// AdminUser is a back-office operator account. There is no separate
// customer/tenant account type yet — out of scope for the P0 slice this
// package implements.
type AdminUser struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"size:64;not null;uniqueIndex"`
	PasswordHash string `gorm:"size:128;not null"`

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
