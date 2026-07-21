package store

import (
	"encoding/json"

	"gorm.io/gorm/clause"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/protocol"
)

// SaveRecords persists recs for deviceID. Duplicate (device_id, ts) pairs
// are silently ignored via ON CONFLICT DO NOTHING against the unique index
// on model.DeviceRecord — this is the "同一(did,ts)去重" requirement,
// enforced by the database instead of an application-level check-then-
// insert (which would race under concurrent uploads).
func (s *Store) SaveRecords(deviceID uint, recs []protocol.Record) error {
	if len(recs) == 0 {
		return nil
	}

	rows := make([]model.DeviceRecord, 0, len(recs))
	for _, r := range recs {
		data, err := json.Marshal(r.D)
		if err != nil {
			return err
		}
		rows = append(rows, model.DeviceRecord{DeviceID: deviceID, Ts: r.Ts, Data: string(data)})
	}

	return s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

// RecentRecords returns a device's most recent telemetry, newest first —
// enough for the admin device-detail page (see internal/api/admin_handlers.go).
// Full historical query/filtering is out of scope for this slice.
func (s *Store) RecentRecords(deviceID uint, limit int) ([]model.DeviceRecord, error) {
	if limit < 1 || limit > 200 {
		limit = 20
	}
	var rows []model.DeviceRecord
	err := s.db.Where("device_id = ?", deviceID).
		Order("ts desc").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}
