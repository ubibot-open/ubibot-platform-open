package rule

import (
	"fmt"
	"log"
	"sync"

	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/models"
	"github.com/ubibot/ubibot-platform-open/internal/telemetry"
)

// AlertNotifier publishes alert events to external systems (e.g. HA).
type AlertNotifier interface {
	PublishAlert(deviceID string, alert map[string]any)
}

// Engine holds alerting rules in memory and evaluates telemetry against them.
type Engine struct {
	db       *gorm.DB
	notifier AlertNotifier
	mu       sync.RWMutex
	rules    map[string][]models.Rule // key: deviceID:metric
}

// New creates a rule engine backed by db. notifier may be nil.
func New(db *gorm.DB, notifier AlertNotifier) *Engine {
	return &Engine{
		db:       db,
		notifier: notifier,
		rules:    make(map[string][]models.Rule),
	}
}

// SetNotifier sets the alert notifier (e.g. HA client). Nil is allowed.
func (e *Engine) SetNotifier(n AlertNotifier) {
	e.mu.Lock()
	e.notifier = n
	e.mu.Unlock()
}

// Load fetches all enabled rules from the database into memory.
func (e *Engine) Load() error {
	var rules []models.Rule
	if err := e.db.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		return fmt.Errorf("load rules: %w", err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = make(map[string][]models.Rule, len(rules))
	for _, r := range rules {
		key := ruleKey(r.DeviceID, r.Metric)
		e.rules[key] = append(e.rules[key], r)
	}
	log.Printf("rule engine loaded %d enabled rules", len(rules))
	return nil
}

// Reload refreshes the in-memory rule set. Call after rule CRUD operations.
func (e *Engine) Reload() error {
	return e.Load()
}

// Match evaluates a telemetry record against loaded rules and triggers
// alerts for any matching rule. Implements telemetry.Sink.
func (e *Engine) Match(r telemetry.Record) {
	e.mu.RLock()
	rules := e.rules[ruleKey(r.DeviceID, r.Metric)]
	notifier := e.notifier
	e.mu.RUnlock()

	for _, rule := range rules {
		if !evaluate(r.Value, rule.Operator, rule.Threshold) {
			continue
		}
		e.trigger(r, rule, notifier)
	}
}

func (e *Engine) trigger(r telemetry.Record, rule models.Rule, notifier AlertNotifier) {
	msg := fmt.Sprintf("%s %s %.2f on device %s", r.Metric, rule.Operator, rule.Threshold, r.DeviceID)
	alert := models.Alert{
		DeviceID: r.DeviceID,
		RuleID:   rule.ID,
		Metric:   r.Metric,
		Value:    r.Value,
		Message:  msg,
	}
	if err := e.db.Create(&alert).Error; err != nil {
		log.Printf("persist alert failed: %v", err)
	}
	log.Printf("alert triggered: %s", msg)
	if notifier != nil {
		notifier.PublishAlert(r.DeviceID, map[string]any{
			"device_id": r.DeviceID,
			"metric":    r.Metric,
			"value":     r.Value,
			"rule_id":   rule.ID,
			"message":   msg,
		})
	}
}

func evaluate(value float64, operator string, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case "<":
		return value < threshold
	case ">=":
		return value >= threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}

func ruleKey(deviceID, metric string) string {
	return deviceID + ":" + metric
}
