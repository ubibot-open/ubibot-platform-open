package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/models"
	"github.com/ubibot/ubibot-platform-open/internal/rule"
)

// RuleAPI exposes rule CRUD endpoints.
type RuleAPI struct {
	db     *gorm.DB
	engine *rule.Engine
}

func NewRuleAPI(db *gorm.DB, engine *rule.Engine) *RuleAPI {
	return &RuleAPI{db: db, engine: engine}
}

// Create adds a new alerting rule and reloads the in-memory rule set.
func (a *RuleAPI) Create(c *gin.Context) {
	var r models.Rule
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !isValidOperator(r.Operator) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid operator"})
		return
	}
	if err := a.db.Create(&r).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = a.engine.Reload()
	c.JSON(http.StatusCreated, r)
}

// List returns rules, optionally filtered by device_id.
func (a *RuleAPI) List(c *gin.Context) {
	q := a.db.Model(&models.Rule{})
	if deviceID := c.Query("device_id"); deviceID != "" {
		q = q.Where("device_id = ?", deviceID)
	}
	var rules []models.Rule
	if err := q.Order("created_at desc").Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

// Get returns a single rule by id.
func (a *RuleAPI) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var r models.Rule
	if err := a.db.First(&r, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}
	c.JSON(http.StatusOK, r)
}

// Update modifies an existing rule and reloads the in-memory rule set.
func (a *RuleAPI) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var r models.Rule
	if err := a.db.First(&r, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}
	var updates models.Rule
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if updates.Operator != "" && !isValidOperator(updates.Operator) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid operator"})
		return
	}
	if err := a.db.Model(&r).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = a.engine.Reload()
	c.JSON(http.StatusOK, r)
}

// Delete removes a rule and reloads the in-memory rule set.
func (a *RuleAPI) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := a.db.Delete(&models.Rule{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = a.engine.Reload()
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

func isValidOperator(op string) bool {
	switch op {
	case ">", "<", ">=", "<=", "==", "!=":
		return true
	}
	return false
}
