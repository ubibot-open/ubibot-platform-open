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

// Package api exposes HTTP REST handlers and route registration for the
// UbiBot platform.
package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/ha"
	"github.com/ubibot/ubibot-platform-open/internal/rule"
	"github.com/ubibot/ubibot-platform-open/internal/telemetry"
)

// NewRouter builds the Gin engine with all REST routes registered.
//
// Route groups:
//   - /device/v1  – device-facing endpoints (telemetry upload, config poll)
//   - /api        – operator/app-facing REST API
func NewRouter(
	db *gorm.DB,
	haClient *ha.Client,
	engine *rule.Engine,
	buf *telemetry.Buffer,
	events DeviceEventSink,
	publisher ConfigPublisher,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	deviceAPI := NewDeviceAPI(db, haClient)
	telemetryAPI := NewTelemetryAPI(db)
	ruleAPI := NewRuleAPI(db, engine)
	alertAPI := NewAlertAPI(db)
	ingestAPI := NewIngestAPI(db, buf, events)
	cfgAPI := NewDeviceConfigAPI(db, publisher)

	// Device-facing endpoints (used by firmware).
	dev := r.Group("/device/v1")
	{
		// Telemetry upload – device POSTs sensor readings, receives config in response.
		dev.POST("/telemetry", ingestAPI.Upload)

		// Config poll – device GETs its current sampling configuration.
		dev.GET("/config/:device_id", cfgAPI.GetDeviceConfig)
	}

	// Operator / app-facing REST API.
	api := r.Group("/api")
	{
		// Devices
		api.POST("/devices", deviceAPI.Register)
		api.GET("/devices", deviceAPI.List)
		api.GET("/devices/:device_id", deviceAPI.Get)

		// Device configuration management
		api.PUT("/devices/:device_id/config", cfgAPI.SetDeviceConfig)
		api.GET("/devices/:device_id/config", cfgAPI.GetDeviceConfig)

		// Telemetry history
		api.GET("/devices/:device_id/telemetry", telemetryAPI.Query)

		// Rules
		api.POST("/rules", ruleAPI.Create)
		api.GET("/rules", ruleAPI.List)
		api.GET("/rules/:id", ruleAPI.Get)
		api.PUT("/rules/:id", ruleAPI.Update)
		api.DELETE("/rules/:id", ruleAPI.Delete)

		// Alerts
		api.GET("/alerts", alertAPI.List)

		// Health
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
	}

	return r
}
