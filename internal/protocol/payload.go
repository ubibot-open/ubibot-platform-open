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

// Package protocol defines the canonical wire-format structures shared by the
// MQTT, CoAP and HTTP device-facing transport layers.
package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

// MaxFields is the number of generic field slots available per data point.
const MaxFields = 20

// FieldKeys returns the canonical JSON key for the 1-based field index n.
// Panics if n is outside [1, MaxFields].
func FieldKey(n int) string {
	if n < 1 || n > MaxFields {
		panic(fmt.Sprintf("field index %d out of range [1,%d]", n, MaxFields))
	}
	return fmt.Sprintf("field%d", n)
}

// DataPoint is a single timed measurement from a device.
// Each upload may contain multiple DataPoints so devices can batch readings
// collected over a longer interval into a single transmission.
//
// All field values are strings. Numeric values should be formatted as their
// decimal representation (e.g. "25.6"); boolean values as "0"/"1" or
// "true"/"false". The platform stores the raw string without conversion.
//
// JSON example:
//
//	{
//	  "timestamp": 1735000000,
//	  "field1": "25.6",
//	  "field2": "60.2",
//	  "field3": "1200"
//	}
type DataPoint struct {
	// Timestamp is the Unix second epoch at which the device collected this
	// reading. The platform substitutes the server receive time when absent.
	Timestamp int64  `json:"timestamp,omitempty"`
	Field1    string `json:"field1,omitempty"`
	Field2    string `json:"field2,omitempty"`
	Field3    string `json:"field3,omitempty"`
	Field4    string `json:"field4,omitempty"`
	Field5    string `json:"field5,omitempty"`
	Field6    string `json:"field6,omitempty"`
	Field7    string `json:"field7,omitempty"`
	Field8    string `json:"field8,omitempty"`
	Field9    string `json:"field9,omitempty"`
	Field10   string `json:"field10,omitempty"`
	Field11   string `json:"field11,omitempty"`
	Field12   string `json:"field12,omitempty"`
	Field13   string `json:"field13,omitempty"`
	Field14   string `json:"field14,omitempty"`
	Field15   string `json:"field15,omitempty"`
	Field16   string `json:"field16,omitempty"`
	Field17   string `json:"field17,omitempty"`
	Field18   string `json:"field18,omitempty"`
	Field19   string `json:"field19,omitempty"`
	Field20   string `json:"field20,omitempty"`
}

// Time returns the data point timestamp as a time.Time, falling back to now.
func (d DataPoint) Time() time.Time {
	if d.Timestamp == 0 {
		return time.Now()
	}
	return time.Unix(d.Timestamp, 0)
}

// Fields returns all non-empty field key→value pairs in this data point.
func (d DataPoint) Fields() map[string]string {
	vals := [MaxFields]string{
		d.Field1, d.Field2, d.Field3, d.Field4, d.Field5,
		d.Field6, d.Field7, d.Field8, d.Field9, d.Field10,
		d.Field11, d.Field12, d.Field13, d.Field14, d.Field15,
		d.Field16, d.Field17, d.Field18, d.Field19, d.Field20,
	}
	m := make(map[string]string, MaxFields)
	for i, v := range vals {
		if v != "" {
			m[fmt.Sprintf("field%d", i+1)] = v
		}
	}
	return m
}

// TelemetryPayload is the canonical uplink message from a device.
// A single upload may carry one or more DataPoints, allowing devices to batch
// readings collected while offline or between upload intervals.
//
// JSON example (two batched readings):
//
//	{
//	  "device_id": "DEV001",
//	  "token":     "abc123...",
//	  "data": [
//	    { "timestamp": 1735000000, "field1": "25.6", "field2": "60.2" },
//	    { "timestamp": 1735000030, "field1": "25.8", "field2": "59.9", "field3": "1180" }
//	  ]
//	}
type TelemetryPayload struct {
	// DeviceID identifies the sender. Required for HTTP and CoAP; for MQTT it
	// may be omitted because the topic already carries the device ID.
	DeviceID string `json:"device_id,omitempty"`

	// Token is the per-device secret used for HTTP/CoAP authentication.
	// MQTT relies on broker-level credentials instead.
	Token string `json:"token,omitempty"`

	// Data holds one or more timed measurements. Must contain at least one
	// DataPoint with at least one non-empty field.
	Data []DataPoint `json:"data"`
}

// ParseTelemetry decodes raw JSON bytes into a TelemetryPayload and performs
// basic validation.
func ParseTelemetry(raw []byte) (*TelemetryPayload, error) {
	var p TelemetryPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("unmarshal telemetry: %w", err)
	}
	if len(p.Data) == 0 {
		return nil, fmt.Errorf("telemetry payload has no data points")
	}
	// Ensure at least one data point has at least one field.
	for _, dp := range p.Data {
		if len(dp.Fields()) > 0 {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("all data points are empty")
}

// ConfigPayload is the downlink message the server sends to a device in
// response to a config request or as a proactive push.
//
// JSON example:
//
//	{
//	  "collect_interval": 30,
//	  "upload_interval":  60,
//	  "fields_enabled": ["field1","field2","field3"],
//	  "server_time":      1735000000
//	}
type ConfigPayload struct {
	// CollectInterval is how often (seconds) the firmware samples its sensors.
	CollectInterval int `json:"collect_interval"`

	// UploadInterval is how often (seconds) the firmware transmits buffered
	// data points to the platform.
	UploadInterval int `json:"upload_interval"`

	// FieldsEnabled lists the field keys the device should collect.
	// An empty slice means "collect all available fields".
	FieldsEnabled []string `json:"fields_enabled,omitempty"`

	// ServerTime is the current Unix epoch stamped by the platform so devices
	// can synchronise their clocks.
	ServerTime int64 `json:"server_time"`
}

// HeartbeatPayload is an optional uplink the device sends to signal liveness
// without uploading a full data package.
type HeartbeatPayload struct {
	DeviceID  string `json:"device_id,omitempty"`
	Token     string `json:"token,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
	// Field values that are cheap to transmit with every heartbeat (e.g. battery, signal).
	Field1 string `json:"field1,omitempty"`
	Field2 string `json:"field2,omitempty"`
}
