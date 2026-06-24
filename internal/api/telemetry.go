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

// telemetry.go implements telemetry query API.
package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/models"
)

// TelemetryAPI exposes telemetry query endpoints.
type TelemetryAPI struct {
	db *gorm.DB
}

func NewTelemetryAPI(db *gorm.DB) *TelemetryAPI {
	return &TelemetryAPI{db: db}
}

// Query returns telemetry records for a device, optionally filtered by
// field key and a time range. Supports pagination via limit/offset.
//
// GET /api/devices/:device_id/telemetry?field=field1&limit=100&offset=0&from=2024-01-01T00:00:00Z&to=2024-01-02T00:00:00Z
func (a *TelemetryAPI) Query(c *gin.Context) {
	deviceID := c.Param("device_id")
	q := a.db.Model(&models.Telemetry{}).Where("device_id = ?", deviceID)

	if field := c.Query("field"); field != "" {
		q = q.Where("field = ?", field)
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			q = q.Where("timestamp >= ?", t)
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			q = q.Where("timestamp <= ?", t)
		}
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	var records []models.Telemetry
	if err := q.Order("timestamp desc").Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, records)
}
