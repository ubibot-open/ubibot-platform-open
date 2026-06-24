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

// ingest.go implements the device-facing HTTP telemetry upload endpoint.
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/models"
	"github.com/ubibot/ubibot-platform-open/internal/protocol"
	"github.com/ubibot/ubibot-platform-open/internal/telemetry"
)

// DeviceEventSink is the subset of coordinator functionality IngestAPI needs.
// Satisfied by *coordinator in cmd/server/main.go.
type DeviceEventSink interface {
	OnTelemetry(deviceID string, payload []byte)
}

// IngestAPI handles the device-facing HTTP telemetry upload endpoint.
type IngestAPI struct {
	db     *gorm.DB
	buf    *telemetry.Buffer
	events DeviceEventSink
}

// NewIngestAPI creates an IngestAPI.
func NewIngestAPI(db *gorm.DB, buf *telemetry.Buffer, events DeviceEventSink) *IngestAPI {
	return &IngestAPI{db: db, buf: buf, events: events}
}

// Upload handles POST /device/v1/telemetry.
//
// Authentication: the device must supply its token in the X-Device-Token
// header or in the JSON body's "token" field.
//
// On success the server returns HTTP 200 with the current device config so the
// device can update its sampling parameters in one round-trip.
func (a *IngestAPI) Upload(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body: " + err.Error()})
		return
	}

	p, err := protocol.ParseTelemetry(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token := c.GetHeader("X-Device-Token")
	if token == "" {
		token = p.Token
	}

	var device models.Device
	if err := a.db.Where("device_id = ?", p.DeviceID).First(&device).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "device not found"})
		return
	}
	if device.Token != token {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	// Feed to shared coordinator path (rule engine + HA + DB).
	a.events.OnTelemetry(p.DeviceID, raw)

	// Return the device config so the device can update its sampling parameters
	// without a separate round-trip.
	cfg := configForDevice(a.db, p.DeviceID)
	c.JSON(http.StatusOK, cfg)
}

// configForDevice loads a device's config from the DB and returns a
// protocol.ConfigPayload ready for wire encoding.
func configForDevice(db *gorm.DB, deviceID string) protocol.ConfigPayload {
	var dc models.DeviceConfig
	if err := db.Where("device_id = ?", deviceID).First(&dc).Error; err != nil {
		// Return sensible defaults when no record exists yet.
		return protocol.ConfigPayload{
			CollectInterval: 30,
			UploadInterval:  60,
			ServerTime:      time.Now().Unix(),
		}
	}

	var enabled []string
	if dc.SensorsEnabled != "" {
		if err := json.Unmarshal([]byte(dc.SensorsEnabled), &enabled); err != nil {
			enabled = nil
		}
	}

	return protocol.ConfigPayload{
		CollectInterval: dc.CollectInterval,
		UploadInterval:  dc.UploadInterval,
		SensorsEnabled:  enabled,
		ServerTime:      time.Now().Unix(),
	}
}
