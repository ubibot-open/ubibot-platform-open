package store

import (
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/model"
)

var ErrNotFound = errors.New("not found")

// MinOfflineGrace is the floor on how long a device can go quiet before
// it's considered offline, regardless of how short its configured upload
// interval is — a device reporting every 5s shouldn't flip offline the
// instant one upload is a few seconds late.
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

// IsDeviceOnline applies the same "quiet for longer than 3x its upload
// interval (floored at the offline-grace floor)" rule the offline-alert
// sweep uses (see OfflineSweep in alert.go) — used here so the admin
// device list/detail view and the alert system never disagree about what
// "online" means.
func IsDeviceOnline(dev *model.Device, now time.Time) bool {
	if dev.LastSeenAt == nil {
		return false
	}
	floor := MinOfflineGrace
	if minOfflineGraceOverride > 0 {
		floor = minOfflineGraceOverride
	}
	grace := time.Duration(dev.UI) * 3 * time.Second
	if grace < floor {
		grace = floor
	}
	return now.Sub(*dev.LastSeenAt) <= grace
}

// CreateDevice provisions a new device row. Secret is stored as given —
// callers (see internal/auth.NewDeviceSecret) are responsible for
// generating something with enough entropy; this layer just persists it.
func (s *Store) CreateDevice(pid, sn, secret, name string) (*model.Device, error) {
	d := &model.Device{
		PID:    pid,
		SN:     sn,
		Secret: secret,
		Name:   name,
		Status: model.DeviceStatusEnabled,
		Source: model.DeviceSourceManual,
		CI:     30,
		UI:     600,
	}
	if err := s.db.Create(d).Error; err != nil {
		return nil, err
	}
	return d, nil
}

// AutoRegisterDevice creates a Pending, self-registered Device row the
// moment an SN the platform has never seen tries to activate (see
// api.Activate). Secret is left empty rather than generated: this is a
// pre-manufactured device with its own factory-set secret already baked
// in, and there is no way to tell it "actually, use this one instead" —
// generating one server-side here would just be a secret the physical
// device can never be told about. It only makes the attempt visible in
// 设备管理 for an operator to review. Nothing about this device can send or
// receive real data until its real secret reaches the platform (docs
// §4.1's key-binding endpoint, or the admin-facing SetDeviceSecret) and
// that flips it to Enabled.
func (s *Store) AutoRegisterDevice(pid, sn string) (*model.Device, error) {
	d := &model.Device{
		PID:    pid,
		SN:     sn,
		Secret: "",
		Status: model.DeviceStatusPending,
		Source: model.DeviceSourceSelfRegistered,
		CI:     30,
		UI:     600,
	}
	if err := s.db.Create(d).Error; err != nil {
		return nil, err
	}
	return d, nil
}

// DeleteDevice permanently removes a device and every record that
// references it — issued tokens, telemetry history, queued commands,
// probes, alert rules/events, its OTA task, and any scheduled task pinned
// to it specifically (a ScheduledTask with DeviceID 0 targets "every
// enabled device" rather than this one, so those are left alone).
// Irreversible; the admin frontend confirms with the operator before
// calling this.
func (s *Store) DeleteDevice(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		deletes := []func() error{
			func() error { return tx.Where("device_id = ?", id).Delete(&model.DeviceToken{}).Error },
			func() error { return tx.Where("device_id = ?", id).Delete(&model.DeviceRecord{}).Error },
			func() error { return tx.Where("device_id = ?", id).Delete(&model.DeviceCommand{}).Error },
			func() error { return tx.Where("device_id = ?", id).Delete(&model.DeviceProbe{}).Error },
			func() error { return tx.Where("device_id = ?", id).Delete(&model.AlertRule{}).Error },
			func() error { return tx.Where("device_id = ?", id).Delete(&model.AlertEvent{}).Error },
			func() error { return tx.Where("device_id = ?", id).Delete(&model.DeviceOTA{}).Error },
			func() error { return tx.Where("device_id = ?", id).Delete(&model.ScheduledTask{}).Error },
		}
		for _, del := range deletes {
			if err := del(); err != nil {
				return err
			}
		}
		return tx.Delete(&model.Device{}, id).Error
	})
}

// SetDeviceSecret records a device's real secret — used by both the
// device-facing key-binding endpoint (api.BindDeviceKey, docs §4.1) and
// the admin-facing "设置密钥" action (api.SetDeviceSecret), the two ways a
// self-registered device's factory-set secret can reach the platform
// after AutoRegisterDevice created it with none. If the device is still
// Pending, this also completes its activation (Pending -> Enabled) in the
// same call — knowing its real secret is the only thing approval was ever
// gating on. A device that isn't Pending (already enabled, disabled, or
// manually created) just gets its secret updated in place, status
// untouched — e.g. correcting a wrong key doesn't silently re-enable a
// device an operator deliberately disabled.
func (s *Store) SetDeviceSecret(id uint, secret string) error {
	var dev model.Device
	if err := s.db.Select("id", "status").First(&dev, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}

	updates := map[string]any{"secret": secret}
	if dev.Status == model.DeviceStatusPending {
		updates["status"] = model.DeviceStatusEnabled
	}
	return s.db.Model(&model.Device{}).Where("id = ?", id).Updates(updates).Error
}

// DeviceBySN looks up a device by serial number. Every device-facing
// handler funnels through this, so "not found" and "found but disabled"
// need to be distinguished by the caller only if it needs to — most
// callers should treat both as auth failure (see docs §8, code 1103).
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
// total row count for pagination.
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

// ListActivatedDevices is like ListDevices but restricted to devices that
// have completed activation (model.Device.Activated) — backing the "数据仓库"
// (data warehouse) view, which is about telemetry and only makes sense for
// devices that have actually gone through the activation handshake and can
// have data. A never-activated device is still visible in ListDevices
// (device management covers provisioning too), just not here.
func (s *Store) ListActivatedDevices(page, pageSize int) ([]model.Device, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	q := s.db.Model(&model.Device{}).Where("activated = ?", true)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var devices []model.Device
	err := q.Order("id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&devices).Error
	if err != nil {
		return nil, 0, err
	}
	return devices, total, nil
}

// SetDeviceConfig updates the sampling/upload config and bumps CfgVersion
// so the next report response includes it (see ProcessReport in
// telemetry.go, which compares CfgVersion against LastSentCfgVersion).
func (s *Store) SetDeviceConfig(id uint, ci, ui int, fe []string) error {
	feJSON, err := json.Marshal(fe)
	if err != nil {
		return err
	}
	return s.db.Model(&model.Device{}).Where("id = ?", id).Updates(map[string]any{
		"ci":          ci,
		"ui":          ui,
		"fe":          string(feJSON),
		"cfg_version": gorm.Expr("cfg_version + 1"),
	}).Error
}

// SetDeviceStatus enables or disables a device (model.DeviceStatusEnabled /
// model.DeviceStatusDisabled).
func (s *Store) SetDeviceStatus(id uint, status int) error {
	return s.db.Model(&model.Device{}).Where("id = ?", id).Update("status", status).Error
}

// MarkDeviceActivated records that a device has completed the activation
// handshake — called once from the /auth/activate handler right after a
// token is issued. Idempotent and one-way: once a device has activated,
// it stays "activated" even if its token later expires or it goes quiet,
// since this tracks activation history, not current session state.
func (s *Store) MarkDeviceActivated(id uint) error {
	return s.db.Model(&model.Device{}).Where("id = ? AND activated = ?", id, false).Update("activated", true).Error
}

// TouchLastSeen records that a device just successfully reported in. now
// is caller-supplied (see api.Server.Now) rather than time.Now() directly
// so the online/offline window this feeds (IsDeviceOnline, OfflineSweep)
// can be driven by a test's mocked clock instead of the wall clock, the
// same pattern the activation-window checks already use.
func (s *Store) TouchLastSeen(id uint, now time.Time) error {
	return s.db.Model(&model.Device{}).Where("id = ?", id).Update("last_seen_at", now).Error
}

// CheckAndAdvanceActivateTS is the anti-replay guard for the "device
// already has a local clock" activation path (protocol §4 note): it
// accepts ts only if it is strictly greater than the last ts recorded for
// this device, and atomically records it if so. This closes the replay
// window a bare ±5-minute check leaves open — a captured signed request
// can't be replayed a second time even within the window, because ts
// won't have advanced.
func (s *Store) CheckAndAdvanceActivateTS(deviceID uint, ts int64) (bool, error) {
	res := s.db.Model(&model.Device{}).
		Where("id = ? AND last_activate_ts < ?", deviceID, ts).
		Update("last_activate_ts", ts)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
