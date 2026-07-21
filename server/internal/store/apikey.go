package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/auth"
	"github.com/ubibot/ubibot-platform-open/internal/model"
)

func hashApiKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// CreateApiKey mints a new key for a third-party integration (see
// internal/api's RequireApiKey, the /api/open/v1 read-only surface). The
// raw key is returned once and never stored — only its hash is, the same
// pattern as a device secret or admin password.
func (s *Store) CreateApiKey(name string) (*model.ApiKey, string, error) {
	raw, err := auth.NewOpaqueToken()
	if err != nil {
		return nil, "", err
	}
	key := &model.ApiKey{Name: name, KeyHash: hashApiKey(raw), Prefix: raw[:8]}
	if err := s.db.Create(key).Error; err != nil {
		return nil, "", err
	}
	return key, raw, nil
}

func (s *Store) ListApiKeys() ([]model.ApiKey, error) {
	var rows []model.ApiKey
	err := s.db.Order("id desc").Find(&rows).Error
	return rows, err
}

func (s *Store) RevokeApiKey(id uint) error {
	return s.db.Model(&model.ApiKey{}).Where("id = ?", id).Update("revoked", true).Error
}

// ValidateApiKey looks up a raw key and returns the owning record if it
// exists and hasn't been revoked, touching LastUsedAt on success.
func (s *Store) ValidateApiKey(raw string) (*model.ApiKey, error) {
	var key model.ApiKey
	err := s.db.Where("key_hash = ? AND revoked = ?", hashApiKey(raw), false).First(&key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	now := time.Now()
	_ = s.db.Model(&key).Update("last_used_at", now).Error
	return &key, nil
}
