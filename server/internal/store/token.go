package store

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/auth"
	"github.com/ubibot/ubibot-platform-open/internal/model"
)

// DeviceTokenTTL is the session token validity period (protocol §4, 86400s
// default).
const DeviceTokenTTL = 24 * time.Hour

// IssueDeviceToken creates a new session token for deviceID, persisting it
// so it survives a restart, and drops any previously issued tokens for the
// same device — one live session per device keeps the table from growing
// without bound and matches "re-activating gets you a fresh token".
func (s *Store) IssueDeviceToken(deviceID uint) (token string, exp time.Duration, err error) {
	tok, err := auth.NewOpaqueToken()
	if err != nil {
		return "", 0, err
	}

	now := time.Now()
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("device_id = ?", deviceID).Delete(&model.DeviceToken{}).Error; err != nil {
			return err
		}
		return tx.Create(&model.DeviceToken{
			Token:     tok,
			DeviceID:  deviceID,
			ExpiresAt: now.Add(DeviceTokenTTL),
		}).Error
	})
	if err != nil {
		return "", 0, err
	}
	return tok, DeviceTokenTTL, nil
}

// DeviceTokenStatus mirrors the outcome the /data/report handler needs to
// turn into protocol error codes 1101/1102 (docs §8).
type DeviceTokenStatus int

const (
	DeviceTokenValid DeviceTokenStatus = iota
	DeviceTokenNotFound
	DeviceTokenExpired
)

// ValidateDeviceToken looks up token and reports its status plus, when
// valid, the owning device and remaining validity (for X-Token-Expires-In).
func (s *Store) ValidateDeviceToken(token string) (*model.Device, time.Duration, DeviceTokenStatus, error) {
	var row model.DeviceToken
	err := s.db.Where("token = ?", token).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, DeviceTokenNotFound, nil
	}
	if err != nil {
		return nil, 0, DeviceTokenNotFound, err
	}

	now := time.Now()
	if now.After(row.ExpiresAt) {
		return nil, 0, DeviceTokenExpired, nil
	}

	dev, err := s.DeviceByID(row.DeviceID)
	if err != nil {
		return nil, 0, DeviceTokenNotFound, err
	}
	return dev, row.ExpiresAt.Sub(now), DeviceTokenValid, nil
}
