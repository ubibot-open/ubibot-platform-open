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

// Known sensor field names used in TelemetryPayload.Sensors.
// Hardware firmware should use these exact keys.
const (
	SensorTemperature    = "temperature"     // °C, float
	SensorHumidity       = "humidity"        // % RH, float
	SensorLight          = "light"           // lux, float
	SensorVibration      = "vibration"       // m/s², float
	SensorSoilNitrogen   = "soil_nitrogen"   // mg/kg, float
	SensorSoilPhosphorus = "soil_phosphorus" // mg/kg, float
	SensorSoilPotassium  = "soil_potassium"  // mg/kg, float
	SensorBattery        = "battery"         // %, float
	SensorSignal         = "signal"          // dBm, int
	SensorPressure       = "pressure"        // hPa, float
	SensorCO2            = "co2"             // ppm, float
)

// TelemetryPayload is the canonical uplink message from a device.
// All three transports (MQTT, HTTP, CoAP) parse into this struct.
//
// JSON example:
//
//	{
//	  "device_id": "DEV001",
//	  "token":     "abc123...",
//	  "timestamp": 1735000000,
//	  "sensors": {
//	    "temperature": 25.6,
//	    "humidity":    60.2,
//	    "light":       1200,
//	    "vibration":   0.02,
//	    "soil_nitrogen":   45,
//	    "soil_phosphorus": 30,
//	    "soil_potassium":  80,
//	    "battery": 85.0,
//	    "signal":  -67
//	  }
//	}
type TelemetryPayload struct {
	// DeviceID identifies the sender. Required for HTTP and CoAP; for MQTT it
	// may be omitted because the topic already carries the device id.
	DeviceID string `json:"device_id,omitempty"`

	// Token is the per-device secret used for HTTP/CoAP authentication.
	// MQTT relies on broker-level credentials instead.
	Token string `json:"token,omitempty"`

	// Timestamp is a Unix second epoch. If zero the server substitutes now().
	Timestamp int64 `json:"timestamp,omitempty"`

	// Sensors holds one or more sensor readings keyed by SensorXxx constants.
	Sensors map[string]json.Number `json:"sensors"`
}

// Time returns the payload timestamp as a time.Time. Falls back to now when
// Timestamp is zero.
func (p *TelemetryPayload) Time() time.Time {
	if p.Timestamp == 0 {
		return time.Now()
	}
	return time.Unix(p.Timestamp, 0)
}

// SensorFloat64 returns the float64 value for the given sensor key together
// with a boolean indicating whether the key was present and parseable.
func (p *TelemetryPayload) SensorFloat64(key string) (float64, bool) {
	n, ok := p.Sensors[key]
	if !ok {
		return 0, false
	}
	f, err := n.Float64()
	return f, err == nil
}

// ParseTelemetry decodes raw JSON bytes into a TelemetryPayload and performs
// basic validation.
func ParseTelemetry(data []byte) (*TelemetryPayload, error) {
	var p TelemetryPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshal telemetry: %w", err)
	}
	if len(p.Sensors) == 0 {
		return nil, fmt.Errorf("telemetry payload has no sensors")
	}
	return &p, nil
}

// ConfigPayload is the downlink message the server sends to a device in
// response to a config request or as a proactive push.
//
// JSON example:
//
//	{
//	  "collect_interval": 30,
//	  "upload_interval":  60,
//	  "sensors_enabled": ["temperature","humidity","light"],
//	  "server_time":      1735000000
//	}
type ConfigPayload struct {
	// CollectInterval is how often (seconds) the firmware samples its sensors.
	CollectInterval int `json:"collect_interval"`

	// UploadInterval is how often (seconds) the firmware transmits buffered
	// readings to the platform.
	UploadInterval int `json:"upload_interval"`

	// SensorsEnabled lists the sensor keys the device should collect.
	// An empty slice means "collect all available sensors".
	SensorsEnabled []string `json:"sensors_enabled,omitempty"`

	// ServerTime is the current Unix epoch the server stamps on every config
	// response so devices can synchronise their clocks.
	ServerTime int64 `json:"server_time"`
}

// HeartbeatPayload is an optional uplink the device sends to signal liveness
// without uploading full telemetry.
type HeartbeatPayload struct {
	DeviceID  string `json:"device_id,omitempty"`
	Token     string `json:"token,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
	Battery   float64 `json:"battery,omitempty"`
	Signal    int    `json:"signal,omitempty"`
}
