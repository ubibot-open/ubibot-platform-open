package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/ha"
	"github.com/ubibot/ubibot-platform-open/internal/rule"
)

// NewRouter builds the Gin engine with all REST routes registered.
func NewRouter(db *gorm.DB, haClient *ha.Client, engine *rule.Engine) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	deviceAPI := NewDeviceAPI(db, haClient)
	telemetryAPI := NewTelemetryAPI(db)
	ruleAPI := NewRuleAPI(db, engine)
	alertAPI := NewAlertAPI(db)

	api := r.Group("/api")
	{
		// Devices
		api.POST("/devices", deviceAPI.Register)
		api.GET("/devices", deviceAPI.List)
		api.GET("/devices/:device_id", deviceAPI.Get)

		// Telemetry
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
