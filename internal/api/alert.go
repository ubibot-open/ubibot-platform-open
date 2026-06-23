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
