package store

import "github.com/ubibot/ubibot-platform-open/internal/model"

// CreateNotification records a system message for the admin header bell
// (消息中心) — called from alert.go (rule/offline triggers) and ota.go
// (upgrade outcomes).
func (s *Store) CreateNotification(typ, level, title, content string) error {
	return s.db.Create(&model.Notification{
		Type: typ, Level: level, Title: title, Content: content,
		Status: model.NotificationStatusUnread,
	}).Error
}

// ListNotifications returns notifications newest first.
func (s *Store) ListNotifications(page, pageSize int) ([]model.Notification, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var total int64
	if err := s.db.Model(&model.Notification{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []model.Notification
	err := s.db.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Store) CountUnreadNotifications() (int64, error) {
	var n int64
	err := s.db.Model(&model.Notification{}).Where("status = ?", model.NotificationStatusUnread).Count(&n).Error
	return n, err
}

func (s *Store) MarkNotificationRead(id uint) error {
	return s.db.Model(&model.Notification{}).Where("id = ?", id).Update("status", model.NotificationStatusRead).Error
}

func (s *Store) MarkAllNotificationsRead() error {
	return s.db.Model(&model.Notification{}).
		Where("status = ?", model.NotificationStatusUnread).
		Update("status", model.NotificationStatusRead).Error
}
