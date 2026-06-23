package models

import "time"

// Device represents a registered UbiBot hardware device.
type Device struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	DeviceID   string     `gorm:"index;column:device_id;not null" json:"device_id"`
	Name       string     `gorm:"column:name" json:"name"`
	Key        string     `gorm:"column:key" json:"key,omitempty"`
	Token      string     `gorm:"column:token" json:"token,omitempty"`
	Online     bool       `gorm:"column:online;default:false" json:"online"`
	LastSeenAt *time.Time `gorm:"column:last_seen_at" json:"last_seen_at,omitempty"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (Device) TableName() string { return "devices" }

// User represents an API consumer or platform administrator.
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"index;column:username;not null" json:"username"`
	PasswordHash string    `gorm:"column:password_hash" json:"-"`
	Token        string    `gorm:"index;column:token" json:"token,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (User) TableName() string { return "users" }

// Rule defines a threshold-based alerting rule for a device metric.
type Rule struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeviceID  string    `gorm:"index;column:device_id;not null" json:"device_id"`
	Metric    string    `gorm:"index;column:metric;not null" json:"metric"`
	Operator  string    `gorm:"column:operator;not null" json:"operator"` // >, <, >=, <=, ==, !=
	Threshold float64   `gorm:"column:threshold" json:"threshold"`
	Action    string    `gorm:"column:action" json:"action"` // alert, ha_event, webhook
	Webhook   string    `gorm:"column:webhook" json:"webhook,omitempty"`
	Enabled   bool      `gorm:"column:enabled;default:true" json:"enabled"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Rule) TableName() string { return "rules" }

// Telemetry stores a single device metric reading. Written in batches.
type Telemetry struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeviceID  string    `gorm:"index;column:device_id;not null" json:"device_id"`
	Metric    string    `gorm:"index;column:metric;not null" json:"metric"`
	Value     float64   `gorm:"column:value" json:"value"`
	Timestamp time.Time `gorm:"index;column:timestamp" json:"timestamp"`
}

func (Telemetry) TableName() string { return "telemetry" }

// Alert records a triggered rule event.
type Alert struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeviceID  string    `gorm:"index;column:device_id;not null" json:"device_id"`
	RuleID    uint      `gorm:"index;column:rule_id" json:"rule_id"`
	Metric    string    `gorm:"column:metric" json:"metric"`
	Value     float64   `gorm:"column:value" json:"value"`
	Message   string    `gorm:"column:message" json:"message"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Alert) TableName() string { return "alerts" }
