package store

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/auth"
	"github.com/ubibot/ubibot-platform-open/internal/model"
)

// AdminSessionTTL is how long an admin login session stays valid before
// requiring a fresh login. There's no refresh flow yet — out of scope for
// this P0 slice.
const AdminSessionTTL = 12 * time.Hour

// CountAdmins is used at startup to decide whether to bootstrap a default
// admin account (see cmd/server/main.go).
func (s *Store) CountAdmins() (int64, error) {
	var n int64
	err := s.db.Model(&model.AdminUser{}).Count(&n).Error
	return n, err
}

// CreateAdmin inserts an admin account with an already-hashed password —
// callers use internal/auth.HashPassword, this layer never sees plaintext.
func (s *Store) CreateAdmin(username, passwordHash string) (*model.AdminUser, error) {
	a := &model.AdminUser{Username: username, PasswordHash: passwordHash}
	if err := s.db.Create(a).Error; err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Store) AdminByUsername(username string) (*model.AdminUser, error) {
	var a model.AdminUser
	if err := s.db.Where("username = ?", username).First(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// IssueAdminSession creates a new bearer token for adminID, dropping any
// previously issued sessions for the same account (one live session per
// admin, same reasoning as IssueDeviceToken).
func (s *Store) IssueAdminSession(adminID uint) (token string, exp time.Duration, err error) {
	tok, err := auth.NewOpaqueToken()
	if err != nil {
		return "", 0, err
	}

	now := time.Now()
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("admin_id = ?", adminID).Delete(&model.AdminSession{}).Error; err != nil {
			return err
		}
		return tx.Create(&model.AdminSession{
			Token:     tok,
			AdminID:   adminID,
			ExpiresAt: now.Add(AdminSessionTTL),
		}).Error
	})
	if err != nil {
		return "", 0, err
	}
	return tok, AdminSessionTTL, nil
}

// ValidateAdminSession looks up token and, if it is unexpired, returns the
// owning admin account.
func (s *Store) ValidateAdminSession(token string) (*model.AdminUser, error) {
	var row model.AdminSession
	if err := s.db.Where("token = ?", token).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if time.Now().After(row.ExpiresAt) {
		return nil, ErrNotFound
	}

	var a model.AdminUser
	if err := s.db.First(&a, row.AdminID).Error; err != nil {
		return nil, err
	}
	return &a, nil
}
