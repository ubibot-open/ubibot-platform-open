package store

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/model"
)

var ErrNotFound = errors.New("not found")

// MinOfflineGrace is how long a device can go quiet before it's considered
// offline. Per docs §4/§6, devices no longer tell the platform their
// upload interval (there's no cfg push anymore), so this is now a single
// fixed floor for every device rather than a per-device multiplier.
const MinOfflineGrace = 2 * time.Minute

// minOfflineGraceOverride lets the "offline_grace_minutes" system
// parameter (see param.go, wired from main.go) raise the floor above the
// MinOfflineGrace default without needing a code change. Zero means "no
// override, use the constant" — tests never touch this, so existing
// online/offline assertions keep their fixed 2-minute floor.
var minOfflineGraceOverride time.Duration

// SetMinOfflineGrace overrides the offline-grace floor at runtime. Pass 0
// to fall back to the MinOfflineGrace constant.
func SetMinOfflineGrace(d time.Duration) {
	minOfflineGraceOverride = d
}

// IsDeviceOnline reports whether dev has reported within the grace period
// — used so the admin device list/detail view and the alert system never
// disagree about what "online" means.
func IsDeviceOnline(dev *model.Device, now time.Time) bool {
	if dev.LastSeenAt == nil {
		return false
	}
	grace := MinOfflineGrace
	if minOfflineGraceOverride > 0 {
		grace = minOfflineGraceOverride
	}
	return now.Sub(*dev.LastSeenAt) <= grace
}

// GetOrCreateDeviceBySN is the entire "provisioning" story per docs §4: a
// device identifies itself with pid+sn and nothing else, so the first
// successful report from an SN the platform hasn't seen creates the row
// on the spot — no admin action, no secret, no pre-registration. created
// is true only when this call is what created the row (useful for
// callers that want to log/audit first contact).
func (s *Store) GetOrCreateDeviceBySN(pid, sn string) (dev *model.Device, created bool, err error) {
	dev, err = s.DeviceBySN(sn)
	if err == nil {
		return dev, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, false, err
	}

	d := &model.Device{
		PID:    pid,
		SN:     sn,
		Status: model.DeviceStatusEnabled,
	}
	if err := s.db.Create(d).Error; err != nil {
		// Likely a race with a concurrent first-contact request for the
		// same SN (unique index) — fetch whichever row won instead of
		// failing the report outright.
		if dev, ferr := s.DeviceBySN(sn); ferr == nil {
			return dev, false, nil
		}
		return nil, false, err
	}
	return d, true, nil
}

// DeleteDevice permanently removes a device and its telemetry/alert
// history. Irreversible; the admin frontend confirms with the operator
// before calling this.
func (s *Store) DeleteDevice(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		deletes := []func() error{
			func() error { return tx.Where("device_id = ?", id).Delete(&model.DeviceRecord{}).Error },
			func() error { return tx.Where("device_id = ?", id).Delete(&model.AlertRule{}).Error },
			func() error { return tx.Where("device_id = ?", id).Delete(&model.AlertEvent{}).Error },
		}
		for _, del := range deletes {
			if err := del(); err != nil {
				return err
			}
		}
		return tx.Delete(&model.Device{}, id).Error
	})
}

// RenameDevice sets a device's display name — the only thing about a
// device an operator can configure after it appears (see docs §6).
func (s *Store) RenameDevice(id uint, name string) error {
	return s.db.Model(&model.Device{}).Where("id = ?", id).Update("name", name).Error
}

// DeviceBySN looks up a device by serial number.
func (s *Store) DeviceBySN(sn string) (*model.Device, error) {
	var d model.Device
	if err := s.db.Where("sn = ?", sn).First(&d).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

func (s *Store) DeviceByID(id uint) (*model.Device, error) {
	var d model.Device
	if err := s.db.First(&d, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

// ListDevices returns a page of devices ordered newest-first, plus the
// total row count for pagination. Every device in this table has, by
// construction (see GetOrCreateDeviceBySN), reported at least once — there
// is no more "provisioned but never activated" state to filter out, so
// this is also what backs 数据仓库 (see api.ListDataWarehouse).
func (s *Store) ListDevices(page, pageSize int) ([]model.Device, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	var total int64
	if err := s.db.Model(&model.Device{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var devices []model.Device
	err := s.db.Order("id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&devices).Error
	if err != nil {
		return nil, 0, err
	}
	return devices, total, nil
}

// SetDeviceStatus enables or disables a device (model.DeviceStatusEnabled /
// model.DeviceStatusDisabled). A disabled device is rejected by every
// device-facing endpoint (see docs §6/§7, code 1103).
func (s *Store) SetDeviceStatus(id uint, status int) error {
	return s.db.Model(&model.Device{}).Where("id = ?", id).Update("status", status).Error
}

// TouchLastSeen records that a device just successfully reported in. now
// is caller-supplied (see api.Server.Now) rather than time.Now() directly
// so the online/offline window this feeds (IsDeviceOnline, OfflineSweep)
// can be driven by a test's mocked clock instead of the wall clock.
func (s *Store) TouchLastSeen(id uint, now time.Time) error {
	return s.db.Model(&model.Device{}).Where("id = ?", id).Update("last_seen_at", now).Error
}
