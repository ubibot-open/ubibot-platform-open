// Package store's alert half: threshold rules evaluated against fresh
// telemetry (this file) and the offline sweep run by a background ticker
// in cmd/server (OfflineSweep below) — the two triggers of model.AlertEvent.
package store

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/protocol"
)

// CreateAlertRule adds a threshold rule for a device.
func (s *Store) CreateAlertRule(deviceID uint, field, op string, threshold float64) (*model.AlertRule, error) {
	rule := &model.AlertRule{DeviceID: deviceID, Field: field, Op: op, Threshold: threshold, Enabled: true}
	if err := s.db.Create(rule).Error; err != nil {
		return nil, err
	}
	return rule, nil
}

// ListAlertRules returns every rule configured for a device.
func (s *Store) ListAlertRules(deviceID uint) ([]model.AlertRule, error) {
	var rows []model.AlertRule
	err := s.db.Where("device_id = ?", deviceID).Order("id asc").Find(&rows).Error
	return rows, err
}

// DeleteAlertRule removes a rule. It does not resolve any alert event the
// rule already triggered — that history stands regardless of whether the
// rule that raised it still exists.
func (s *Store) DeleteAlertRule(id uint) error {
	return s.db.Delete(&model.AlertRule{}, id).Error
}

// AlertFilter narrows ListAlertEvents — zero values mean "don't filter".
type AlertFilter struct {
	DeviceID uint
	Status   string
}

// ListAlertEvents returns alert events across devices, newest first.
func (s *Store) ListAlertEvents(f AlertFilter, page, pageSize int) ([]model.AlertEvent, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	scope := func(db *gorm.DB) *gorm.DB {
		if f.DeviceID != 0 {
			db = db.Where("device_id = ?", f.DeviceID)
		}
		if f.Status != "" {
			db = db.Where("status = ?", f.Status)
		}
		return db
	}

	var total int64
	if err := scope(s.db.Model(&model.AlertEvent{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []model.AlertEvent
	err := scope(s.db).Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// ResolveAlertEvent manually marks an open alert as resolved (e.g. an
// operator acknowledging it) independent of whether the underlying
// condition actually cleared.
func (s *Store) ResolveAlertEvent(id uint) error {
	now := time.Now()
	return s.db.Model(&model.AlertEvent{}).Where("id = ? AND status = ?", id, model.AlertStatusOpen).
		Updates(map[string]any{"status": model.AlertStatusResolved, "resolved_at": now}).Error
}

// openAlertFor finds an already-open event for (deviceID, ruleID) so
// evaluateThresholdRules doesn't create duplicate rows every time a
// still-violating reading comes in.
func (s *Store) openAlertFor(deviceID, ruleID uint) (*model.AlertEvent, error) {
	var ev model.AlertEvent
	err := s.db.Where("device_id = ? AND rule_id = ? AND status = ?", deviceID, ruleID, model.AlertStatusOpen).
		First(&ev).Error
	if err != nil {
		return nil, err
	}
	return &ev, nil
}

func compare(op string, value, threshold float64) bool {
	switch op {
	case model.AlertOpGT:
		return value > threshold
	case model.AlertOpGE:
		return value >= threshold
	case model.AlertOpLT:
		return value < threshold
	case model.AlertOpLE:
		return value <= threshold
	case model.AlertOpEQ:
		return value == threshold
	default:
		return false
	}
}

// evaluateThresholdRules checks the latest value of each field touched by
// this report's records against the device's enabled AlertRules, opening
// a new AlertEvent the first time a rule starts violating and
// auto-resolving it the first time a later reading stops violating —
// callers don't need to do anything themselves to "clear" an alert.
func (s *Store) evaluateThresholdRules(deviceID uint, recs []protocol.Record) error {
	if len(recs) == 0 {
		return nil
	}

	latest := make(map[string]float64)
	for _, r := range recs {
		for field, v := range r.D {
			if f, ok := toFloat(v); ok {
				latest[field] = f
			}
		}
	}
	if len(latest) == 0 {
		return nil
	}

	rules, err := s.ListAlertRules(deviceID)
	if err != nil {
		return err
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		value, ok := latest[rule.Field]
		if !ok {
			continue
		}

		violating := compare(rule.Op, value, rule.Threshold)
		existing, err := s.openAlertFor(deviceID, rule.ID)
		hasOpen := err == nil

		switch {
		case violating && !hasOpen:
			ev := &model.AlertEvent{
				DeviceID:    deviceID,
				RuleID:      rule.ID,
				Type:        model.AlertTypeThreshold,
				Message:     fmt.Sprintf("%s %s %g（当前值 %g）", rule.Field, rule.Op, rule.Threshold, value),
				Status:      model.AlertStatusOpen,
				TriggeredAt: time.Now(),
			}
			if err := s.db.Create(ev).Error; err != nil {
				return err
			}
		case !violating && hasOpen:
			now := time.Now()
			if err := s.db.Model(existing).Updates(map[string]any{
				"status": model.AlertStatusResolved, "resolved_at": now,
			}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}

// OfflineSweep is run periodically (see cmd/server main.go) rather than
// on report — offline is the absence of a report, so nothing about
// receiving one can detect it. Uses the same IsDeviceOnline rule the
// admin device list/detail view displays, so the alert center and the UI
// never disagree about which devices are offline.
func (s *Store) OfflineSweep(now time.Time) error {
	var devices []model.Device
	if err := s.db.Where("status = ?", model.DeviceStatusEnabled).Find(&devices).Error; err != nil {
		return err
	}

	for _, dev := range devices {
		offline := !IsDeviceOnline(&dev, now)
		var existing model.AlertEvent
		err := s.db.Where("device_id = ? AND type = ? AND status = ?", dev.ID, model.AlertTypeOffline, model.AlertStatusOpen).
			First(&existing).Error
		hasOpen := err == nil

		switch {
		case offline && !hasOpen:
			ev := &model.AlertEvent{
				DeviceID:    dev.ID,
				Type:        model.AlertTypeOffline,
				Message:     "设备离线：超过预期上报间隔未收到数据",
				Status:      model.AlertStatusOpen,
				TriggeredAt: now,
			}
			if err := s.db.Create(ev).Error; err != nil {
				return err
			}
		case !offline && hasOpen:
			if err := s.db.Model(&existing).Updates(map[string]any{
				"status": model.AlertStatusResolved, "resolved_at": now,
			}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
