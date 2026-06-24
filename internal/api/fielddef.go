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

// fielddef.go implements CRUD endpoints for FieldDefinition records.
// Operators use these to assign human-readable names and units to the generic
// field1..field20 slots reported by device firmware.
package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/models"
)

// FieldDefAPI manages field definitions.
type FieldDefAPI struct {
	db *gorm.DB
}

// NewFieldDefAPI creates a FieldDefAPI.
func NewFieldDefAPI(db *gorm.DB) *FieldDefAPI {
	return &FieldDefAPI{db: db}
}

type upsertFieldDefRequest struct {
	DisplayName string `json:"display_name"`
	Unit        string `json:"unit"`
	Description string `json:"description"`
}

// ListFieldDefs handles GET /api/field-definitions
// Returns all field definitions, optionally filtered by ?device_id=.
func (a *FieldDefAPI) ListFieldDefs(c *gin.Context) {
	deviceID := c.Query("device_id")
	query := a.db.Order("device_id, field_key")
	if deviceID != "" {
		query = query.Where("device_id = ? OR device_id = ''", deviceID)
	}
	var defs []models.FieldDefinition
	if err := query.Find(&defs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, defs)
}

// SetFieldDef handles PUT /api/field-definitions/:device_id/:field_key
// Creates or updates the definition for one field slot on a specific device.
// Use device_id "_default" to set a global default visible to all devices.
func (a *FieldDefAPI) SetFieldDef(c *gin.Context) {
	rawDeviceID := c.Param("device_id")
	fieldKey := strings.ToLower(c.Param("field_key"))

	if err := validateFieldKey(fieldKey); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// "_default" maps to the empty device_id sentinel used for global defs.
	deviceID := rawDeviceID
	if rawDeviceID == "_default" {
		deviceID = ""
	}

	var req upsertFieldDefRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var def models.FieldDefinition
	result := a.db.Where("device_id = ? AND field_key = ?", deviceID, fieldKey).First(&def)
	if result.Error != nil {
		def = models.FieldDefinition{DeviceID: deviceID, FieldKey: fieldKey}
	}
	def.DisplayName = req.DisplayName
	def.Unit = req.Unit
	def.Description = req.Description
	def.UpdatedAt = time.Now()

	if err := a.db.Save(&def).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, def)
}

// DeleteFieldDef handles DELETE /api/field-definitions/:device_id/:field_key
func (a *FieldDefAPI) DeleteFieldDef(c *gin.Context) {
	rawDeviceID := c.Param("device_id")
	fieldKey := strings.ToLower(c.Param("field_key"))

	deviceID := rawDeviceID
	if rawDeviceID == "_default" {
		deviceID = ""
	}

	if err := a.db.Where("device_id = ? AND field_key = ?", deviceID, fieldKey).
		Delete(&models.FieldDefinition{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// validateFieldKey checks that key is "field1".."field20".
func validateFieldKey(key string) error {
	var n int
	if _, err := fmt.Sscanf(key, "field%d", &n); err != nil || n < 1 || n > 20 {
		return fmt.Errorf("field_key must be field1..field20, got %q", key)
	}
	return nil
}
