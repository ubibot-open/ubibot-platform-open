package store

import (
	"errors"

	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/model"
)

// ListIcons returns every custom field icon, key ascending. There is no
// pagination story here -- this is a small admin-curated set (at most one
// row per sensor field name), not user-generated volume.
func (s *Store) ListIcons() ([]model.IconAsset, error) {
	var rows []model.IconAsset
	err := s.db.Order("key asc").Find(&rows).Error
	return rows, err
}

// UpsertIcon creates or replaces the icon for key. Uploading again for a key
// already in the library overwrites its name/SVG rather than erroring --
// "re-upload to change it" is the expected workflow (see
// admin_handlers.go's UploadIcon).
func (s *Store) UpsertIcon(key, name, svg string) (*model.IconAsset, error) {
	var existing model.IconAsset
	err := s.db.Where("key = ?", key).First(&existing).Error
	if err == nil {
		existing.Name = name
		existing.SVG = svg
		if err := s.db.Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	row := &model.IconAsset{Key: key, Name: name, SVG: svg}
	if err := s.db.Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// DeleteIcon removes the icon for key, if any. Deleting a key with no
// custom icon is a no-op, not an error -- the caller (数据仓库 page) just
// keeps using its built-in default for that field.
func (s *Store) DeleteIcon(key string) error {
	return s.db.Where("key = ?", key).Delete(&model.IconAsset{}).Error
}
