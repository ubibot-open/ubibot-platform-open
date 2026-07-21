package store

import (
	"strconv"

	"gorm.io/gorm/clause"

	"github.com/ubibot/ubibot-platform-open/internal/model"
)

// Well-known system parameter keys. Not every parameter that could exist
// has to be listed here — only the ones internal/api and cmd/server
// actually read back into live behavior (see param_handlers.go and
// main.go's applySystemParams).
const (
	ParamRateLimitPerMinute = "rate_limit_per_minute"
	ParamOfflineGraceMinute = "offline_grace_minutes"
)

// SetParam upserts a parameter by key.
func (s *Store) SetParam(key, value, description string) (*model.SystemParam, error) {
	p := &model.SystemParam{Key: key, Value: value, Description: description}
	err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "description"}),
	}).Create(p).Error
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) GetParam(key string) (string, bool) {
	var p model.SystemParam
	if err := s.db.Where("key = ?", key).First(&p).Error; err != nil {
		return "", false
	}
	return p.Value, true
}

// GetParamInt reads a parameter as an int, falling back to def if it's
// unset or not parseable — callers should never fail startup or a
// request over a malformed parameter value.
func (s *Store) GetParamInt(key string, def int) int {
	v, ok := s.GetParam(key)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func (s *Store) ListParams() ([]model.SystemParam, error) {
	var rows []model.SystemParam
	err := s.db.Order("key asc").Find(&rows).Error
	return rows, err
}
