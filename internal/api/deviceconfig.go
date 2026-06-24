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

// deviceconfig.go manages per-device configuration (collect/upload intervals,
// enabled sensors) via the REST API and propagates changes via MQTT.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/models"
	"github.com/ubibot/ubibot-platform-open/internal/protocol"
)

// ConfigPublisher can push a config payload down to a device over MQTT.
// Satisfied by *mqttbroker.Broker.
type ConfigPublisher interface {
	Publish(topic string, payload []byte, retain bool) error
}

// DeviceConfigAPI manages device configuration.
type DeviceConfigAPI struct {
	db        *gorm.DB
	publisher ConfigPublisher // may be nil when MQTT is disabled
}

// NewDeviceConfigAPI creates a DeviceConfigAPI.
func NewDeviceConfigAPI(db *gorm.DB, publisher ConfigPublisher) *DeviceConfigAPI {
	return &DeviceConfigAPI{db: db, publisher: publisher}
}

// GetDeviceConfig handles GET /device/v1/config/:device_id
// Used by devices to poll their configuration on startup or after reconnect.
func (a *DeviceConfigAPI) GetDeviceConfig(c *gin.Context) {
	deviceID := c.Param("device_id")
	cfg := configForDevice(a.db, deviceID)
	c.JSON(http.StatusOK, cfg)
}

type setConfigRequest struct {
	CollectInterval int      `json:"collect_interval"`
	UploadInterval  int      `json:"upload_interval"`
	SensorsEnabled  []string `json:"sensors_enabled"`
}

// SetDeviceConfig handles PUT /api/devices/:device_id/config
// Platform operators call this to update a device's sampling parameters.
// The new config is persisted and immediately pushed over MQTT (retained).
func (a *DeviceConfigAPI) SetDeviceConfig(c *gin.Context) {
	deviceID := c.Param("device_id")

	var req setConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.CollectInterval < 1 {
		req.CollectInterval = 30
	}
	if req.UploadInterval < 1 {
		req.UploadInterval = 60
	}

	enabledJSON, _ := json.Marshal(req.SensorsEnabled)

	var dc models.DeviceConfig
	result := a.db.Where("device_id = ?", deviceID).First(&dc)
	if result.Error != nil {
		dc = models.DeviceConfig{DeviceID: deviceID}
	}
	dc.CollectInterval = req.CollectInterval
	dc.UploadInterval = req.UploadInterval
	dc.SensorsEnabled = string(enabledJSON)
	dc.UpdatedAt = time.Now()

	if err := a.db.Save(&dc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	payload := protocol.ConfigPayload{
		CollectInterval: dc.CollectInterval,
		UploadInterval:  dc.UploadInterval,
		SensorsEnabled:  req.SensorsEnabled,
		ServerTime:      time.Now().Unix(),
	}

	if a.publisher != nil {
		data, _ := json.Marshal(payload)
		topic := "ubibot/" + deviceID + "/cmd/config"
		_ = a.publisher.Publish(topic, data, true) // retained so device gets it on reconnect
	}

	c.JSON(http.StatusOK, payload)
}
