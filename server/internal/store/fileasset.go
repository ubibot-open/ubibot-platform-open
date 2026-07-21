package store

import (
	"errors"
	"os"

	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/model"
)

// CreateFileAsset records a generic uploaded file (exports, attachments,
// etc. — separate from Firmware, which has its own OTA-specific columns).
// The caller has already written the bytes to path.
func (s *Store) CreateFileAsset(category, filename, path string, size int64, sha256 string) (*model.FileAsset, error) {
	f := &model.FileAsset{Category: category, Filename: filename, Path: path, Size: size, SHA256: sha256}
	if err := s.db.Create(f).Error; err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Store) ListFileAssets() ([]model.FileAsset, error) {
	var rows []model.FileAsset
	err := s.db.Order("id desc").Find(&rows).Error
	return rows, err
}

func (s *Store) FileAssetByID(id uint) (*model.FileAsset, error) {
	var f model.FileAsset
	if err := s.db.First(&f, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &f, nil
}

// DeleteFileAsset removes both the DB record and the on-disk file
// (best-effort — a missing file shouldn't block cleaning up the record).
func (s *Store) DeleteFileAsset(id uint) error {
	f, err := s.FileAssetByID(id)
	if err != nil {
		return err
	}
	_ = os.Remove(f.Path)
	return s.db.Delete(&model.FileAsset{}, id).Error
}
