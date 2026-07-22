package store

import (
	"encoding/json"

	"gorm.io/gorm"
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

// LatestRecordsByDevice returns, for each ID in deviceIDs that has at least
// one stored record, only that device's single most recent DeviceRecord —
// one query instead of len(deviceIDs) separate ones, via a window function
// over the existing (device_id, ts) index. Backs the "数据仓库" list's
// per-row sensor-data preview.
func (s *Store) LatestRecordsByDevice(deviceIDs []uint) (map[uint]model.DeviceRecord, error) {
	result := make(map[uint]model.DeviceRecord, len(deviceIDs))
	if len(deviceIDs) == 0 {
		return result, nil
	}

	var rows []model.DeviceRecord
	err := s.db.Raw(`
		SELECT id, device_id, ts, data, created_at FROM (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY device_id ORDER BY ts DESC) AS rn
			FROM device_records
			WHERE device_id IN ?
		) WHERE rn = 1
	`, deviceIDs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.DeviceID] = r
	}
	return result, nil
}

// RecentRecords returns a device's most recent telemetry, newest first —
// used by the admin device-detail page's "最近上报数据" panel.
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

// QueryRecords returns a device's telemetry within [start, end] (Unix
// seconds; either may be 0 to leave that bound open), oldest first — this
// is the "历史数据查询" page's backing query, as opposed to RecentRecords'
// fixed newest-first snapshot for the detail view.
func (s *Store) QueryRecords(deviceID uint, start, end int64, page, pageSize int) ([]model.DeviceRecord, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 100
	}

	scope := func(db *gorm.DB) *gorm.DB {
		db = db.Where("device_id = ?", deviceID)
		if start > 0 {
			db = db.Where("ts >= ?", start)
		}
		if end > 0 {
			db = db.Where("ts <= ?", end)
		}
		return db
	}

	var total int64
	if err := scope(s.db.Model(&model.DeviceRecord{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []model.DeviceRecord
	err := scope(s.db).Order("ts asc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// CountRecordsSince returns how many telemetry rows have ts >= since —
// used by the dashboard summary for "今日上报条数".
func (s *Store) CountRecordsSince(since int64) (int64, error) {
	var n int64
	err := s.db.Model(&model.DeviceRecord{}).Where("ts >= ?", since).Count(&n).Error
	return n, err
}

// DailyRecordCount is one bucket of RecordCountsByDay's result.
type DailyRecordCount struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

// RecordCountsByDay buckets telemetry rows by calendar day (UTC) for
// ts >= since — the dashboard trend chart's backing query.
func (s *Store) RecordCountsByDay(since int64) ([]DailyRecordCount, error) {
	var rows []DailyRecordCount
	err := s.db.Model(&model.DeviceRecord{}).
		Select("date(ts, 'unixepoch') as day, count(*) as count").
		Where("ts >= ?", since).
		Group("day").
		Order("day asc").
		Scan(&rows).Error
	return rows, err
}
