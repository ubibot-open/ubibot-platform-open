package store

import "github.com/ubibot/ubibot-platform-open/internal/model"

// WriteAudit records one mutating admin action. Called from
// internal/api's writeAudit helper at the point a handler succeeds —
// read-only endpoints aren't audited, only things that changed state.
func (s *Store) WriteAudit(adminID uint, username, action, targetType string, targetID uint, detail, ip string) error {
	return s.db.Create(&model.AuditLog{
		AdminID:    adminID,
		Username:   username,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     detail,
		IP:         ip,
	}).Error
}

// ListAuditLogs returns audit entries, newest first.
func (s *Store) ListAuditLogs(page, pageSize int) ([]model.AuditLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	var total int64
	if err := s.db.Model(&model.AuditLog{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []model.AuditLog
	err := s.db.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
