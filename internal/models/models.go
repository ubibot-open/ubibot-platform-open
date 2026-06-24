// Copyright 2026 UbiBot Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package models defines the GORM data models shared across the platform.
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

// Rule defines a threshold-based alerting rule for a device field.
// The Metric column stores a field key ("field1".."field20").
// Threshold comparison is performed by parsing the stored string value as
// float64; non-numeric values are treated as 0.
type Rule struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeviceID  string    `gorm:"index;column:device_id;not null" json:"device_id"`
	Metric    string    `gorm:"index;column:metric;not null" json:"metric"` // "field1".."field20"
	Operator  string    `gorm:"column:operator;not null" json:"operator"`   // >, <, >=, <=, ==, !=
	Threshold float64   `gorm:"column:threshold" json:"threshold"`
	Action    string    `gorm:"column:action" json:"action"` // alert, ha_event, webhook
	Webhook   string    `gorm:"column:webhook" json:"webhook,omitempty"`
	Enabled   bool      `gorm:"column:enabled;default:true" json:"enabled"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Rule) TableName() string { return "rules" }

// FieldDefinition maps a generic field key ("field1".."field20") to a
// human-readable name, unit and description. Definitions are per-device;
// a record with an empty DeviceID serves as the global default for all
// devices that do not have an explicit entry.
type FieldDefinition struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	DeviceID    string    `gorm:"index;column:device_id" json:"device_id"` // empty = global default
	FieldKey    string    `gorm:"index;column:field_key;not null" json:"field_key"` // "field1".."field20"
	DisplayName string    `gorm:"column:display_name" json:"display_name"` // e.g. "Temperature"
	Unit        string    `gorm:"column:unit" json:"unit"`                 // e.g. "°C"
	Description string    `gorm:"column:description" json:"description"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (FieldDefinition) TableName() string { return "field_definitions" }

// Telemetry stores a single device field reading. Written in batches.
// Value is stored as a raw string exactly as received from the device.
// The platform does not coerce the type; consumers parse as needed.
type Telemetry struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeviceID  string    `gorm:"index;column:device_id;not null" json:"device_id"`
	Field     string    `gorm:"index;column:field;not null" json:"field"` // "field1".."field20"
	Value     string    `gorm:"column:value" json:"value"`                // raw string from device
	Timestamp time.Time `gorm:"index;column:timestamp" json:"timestamp"`
}

func (Telemetry) TableName() string { return "telemetry" }

// Alert records a triggered rule event.
type Alert struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeviceID  string    `gorm:"index;column:device_id;not null" json:"device_id"`
	RuleID    uint      `gorm:"index;column:rule_id" json:"rule_id"`
	Field     string    `gorm:"column:field" json:"field"` // "field1".."field20"
	Value     string    `gorm:"column:value" json:"value"` // raw string value that triggered the rule
	Message   string    `gorm:"column:message" json:"message"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Alert) TableName() string { return "alerts" }

// DeviceConfig stores the configuration the platform pushes down to a device.
// When a device polls /device/v1/config or the MQTT cmd/config topic, this
// record is serialised as a protocol.ConfigPayload.
type DeviceConfig struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	DeviceID        string    `gorm:"uniqueIndex;column:device_id;not null" json:"device_id"`
	CollectInterval int       `gorm:"column:collect_interval;default:30" json:"collect_interval"`
	UploadInterval  int       `gorm:"column:upload_interval;default:60" json:"upload_interval"`
	FieldsEnabled   string    `gorm:"column:fields_enabled" json:"fields_enabled"` // JSON array, empty = all
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (DeviceConfig) TableName() string { return "device_configs" }
