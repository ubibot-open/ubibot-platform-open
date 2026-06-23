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

// device.go implements device registration and management API.
package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/ha"
	"github.com/ubibot/ubibot-platform-open/internal/models"
)

// DeviceAPI exposes device registration and management endpoints.
type DeviceAPI struct {
	db       *gorm.DB
	haClient *ha.Client
}

// NewDeviceAPI creates a device API handler. haClient may be nil when
// Home Assistant integration is disabled.
func NewDeviceAPI(db *gorm.DB, haClient *ha.Client) *DeviceAPI {
	return &DeviceAPI{db: db, haClient: haClient}
}

type registerDeviceRequest struct {
	DeviceID string `json:"device_id" binding:"required"`
	Name     string `json:"name"`
}

// Register creates a new device and returns it with a generated token.
// HA discovery is published when integration is enabled.
func (a *DeviceAPI) Register(c *gin.Context) {
	var req registerDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := generateToken(32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generate token"})
		return
	}

	device := models.Device{
		DeviceID: req.DeviceID,
		Name:     req.Name,
		Token:    token,
		Online:   false,
	}
	if err := a.db.Create(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if a.haClient != nil {
		a.haClient.PublishDeviceDiscovery(device.DeviceID, device.Name)
	}

	c.JSON(http.StatusCreated, device)
}

// List returns all registered devices.
func (a *DeviceAPI) List(c *gin.Context) {
	var devices []models.Device
	if err := a.db.Order("created_at desc").Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, devices)
}

// Get returns a single device by device_id.
func (a *DeviceAPI) Get(c *gin.Context) {
	deviceID := c.Param("device_id")
	var device models.Device
	if err := a.db.Where("device_id = ?", deviceID).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	c.JSON(http.StatusOK, device)
}

// UpdateStatus sets the online flag and last_seen_at for a device.
func (a *DeviceAPI) UpdateStatus(deviceID string, online bool) {
	now := time.Now()
	updates := map[string]any{
		"online":       online,
		"last_seen_at": now,
	}
	a.db.Model(&models.Device{}).Where("device_id = ?", deviceID).Updates(updates)
}

func generateToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
