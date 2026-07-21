package store

import "github.com/ubibot/ubibot-platform-open/internal/model"

func (s *Store) CreateDictEntry(typ, key, label string, sort int) (*model.DictEntry, error) {
	e := &model.DictEntry{Type: typ, Key: key, Label: label, Sort: sort}
	if err := s.db.Create(e).Error; err != nil {
		return nil, err
	}
	return e, nil
}

// ListDictEntries returns entries for typ (or every entry if typ is ""),
// ordered for direct use as a dropdown's option list.
func (s *Store) ListDictEntries(typ string) ([]model.DictEntry, error) {
	q := s.db.Order("sort asc, id asc")
	if typ != "" {
		q = q.Where("type = ?", typ)
	}
	var rows []model.DictEntry
	err := q.Find(&rows).Error
	return rows, err
}

func (s *Store) UpdateDictEntry(id uint, label string, sort int) error {
	return s.db.Model(&model.DictEntry{}).Where("id = ?", id).Updates(map[string]any{
		"label": label, "sort": sort,
	}).Error
}

func (s *Store) DeleteDictEntry(id uint) error {
	return s.db.Delete(&model.DictEntry{}, id).Error
}
