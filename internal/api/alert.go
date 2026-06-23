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

// alert.go implements the alert query API.
package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/models"
)

// AlertAPI exposes alert query endpoints.
type AlertAPI struct {
	db *gorm.DB
}

func NewAlertAPI(db *gorm.DB) *AlertAPI {
	return &AlertAPI{db: db}
}

// List returns alerts, optionally filtered by device_id.
func (a *AlertAPI) List(c *gin.Context) {
	q := a.db.Model(&models.Alert{})
	if deviceID := c.Query("device_id"); deviceID != "" {
		q = q.Where("device_id = ?", deviceID)
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	var alerts []models.Alert
	if err := q.Order("created_at desc").Limit(limit).Find(&alerts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, alerts)
}
