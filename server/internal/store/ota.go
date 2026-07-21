// Package store's OTA half (protocol §7.3): firmware metadata management
// plus the per-device upgrade state machine driven by the device's own
// progress reports (applyOtaStatus) and, as a fallback, its ack/nak of the
// ota command itself (applyOtaCommandOutcome, wired in from report.go).
package store

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/protocol"
)

// CreateFirmware records an uploaded firmware image. path is where the
// binary was already written to disk by the caller (see
// internal/api/ota_admin_handlers.go) — this layer only tracks metadata.
func (s *Store) CreateFirmware(pid, version, filename, path string, size int64, sha256, signature string) (*model.Firmware, error) {
	fw := &model.Firmware{
		PID: pid, Version: version, Filename: filename, Path: path,
		Size: size, SHA256: sha256, Signature: signature,
	}
	if err := s.db.Create(fw).Error; err != nil {
		return nil, err
	}
	return fw, nil
}

func (s *Store) ListFirmware() ([]model.Firmware, error) {
	var rows []model.Firmware
	err := s.db.Order("id desc").Find(&rows).Error
	return rows, err
}

func (s *Store) FirmwareByID(id uint) (*model.Firmware, error) {
	var fw model.Firmware
	if err := s.db.First(&fw, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &fw, nil
}

// DeleteFirmware removes the metadata row only — callers are responsible
// for removing the on-disk file (see internal/api, which does so best-
// effort since a missing file shouldn't block cleaning up the record).
func (s *Store) DeleteFirmware(id uint) error {
	return s.db.Delete(&model.Firmware{}, id).Error
}

// DispatchOTA queues an ota(action=start) command for deviceID and
// records a DeviceOTA row in "pending" state — flipped to downloading/
// verifying/etc. as the device reports progress (see applyOtaStatus).
// Dispatching again while one is already in flight simply overwrites the
// tracking row; only one upgrade is meaningful per device at a time.
func (s *Store) DispatchOTA(deviceID uint, fw *model.Firmware, downloadURL string, force bool) (*model.DeviceCommand, error) {
	cmd, err := s.QueueCommand(deviceID, "ota", map[string]interface{}{
		"action":  "start",
		"version": fw.Version,
		"url":     downloadURL,
		"size":    fw.Size,
		"sha256":  fw.SHA256,
		"sig":     fw.Signature,
		"force":   force,
	})
	if err != nil {
		return nil, err
	}

	ota := &model.DeviceOTA{
		DeviceID: deviceID, FirmwareID: fw.ID, CmdID: cmd.CmdID, Version: fw.Version,
		State: model.OtaStatePending,
	}
	err = s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "device_id"}},
		DoUpdates: clause.AssignmentColumns(
			[]string{"firmware_id", "cmd_id", "version", "state", "progress", "last_error"}),
	}).Create(ota).Error
	if err != nil {
		return nil, err
	}
	return cmd, nil
}

// CancelOTA queues an ota(action=cancel) command referencing the device's
// current in-flight version — per protocol §7.3 the device itself decides
// whether it's still safe to abort (before flashing) or must nak the
// cancel and continue (after).
func (s *Store) CancelOTA(deviceID uint) (*model.DeviceCommand, error) {
	ota, err := s.DeviceOTAByDevice(deviceID)
	if err != nil {
		return nil, err
	}
	return s.QueueCommand(deviceID, "ota", map[string]interface{}{"action": "cancel", "version": ota.Version})
}

func (s *Store) DeviceOTAByDevice(deviceID uint) (*model.DeviceOTA, error) {
	var ota model.DeviceOTA
	if err := s.db.Where("device_id = ?", deviceID).First(&ota).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &ota, nil
}

var terminalOtaStates = map[string]bool{
	model.OtaStateSuccess:    true,
	model.OtaStateFailed:     true,
	model.OtaStateRolledBack: true,
}

// applyOtaStatus records the device's self-reported OTA progress
// (protocol §7.3's "ota" report field) — the authoritative source for
// state/progress while the upgrade is running. Once a terminal state has
// been recorded, later reports (e.g. from a retried/duplicated upload)
// can't reopen it.
func (s *Store) applyOtaStatus(deviceID uint, ota *protocol.OtaStatus) error {
	if ota == nil {
		return nil
	}
	var existing model.DeviceOTA
	err := s.db.Where("device_id = ? AND cmd_id = ?", deviceID, ota.ID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil // status for a cmd this server has no device_otas row for (stale/unknown) — ignore rather than error the whole report
	}
	if err != nil {
		return err
	}
	if terminalOtaStates[existing.State] {
		return nil
	}

	if err := s.db.Model(&existing).Updates(map[string]any{
		"state": ota.State, "progress": ota.Progress,
	}).Error; err != nil {
		return err
	}
	if terminalOtaStates[ota.State] {
		s.notifyOtaOutcome(deviceID, ota.State, ota.Version)
	}
	return nil
}

// applyOtaCommandOutcome mirrors applyProbeCommandOutcome for the "ota"
// cmd type: a fallback that finalizes state via plain ack/nak only if the
// device's own status reports never reached a terminal value (e.g. it
// crashed or lost connectivity right after flashing, before it could
// report "success" itself, but still eventually acked the cmd).
func (s *Store) applyOtaCommandOutcome(deviceID uint, cmdID string, acked bool, nakMessage string) error {
	var existing model.DeviceOTA
	err := s.db.Where("device_id = ? AND cmd_id = ?", deviceID, cmdID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if terminalOtaStates[existing.State] {
		return nil
	}

	state := model.OtaStateFailed
	if acked {
		state = model.OtaStateSuccess
	}
	if err := s.db.Model(&existing).Updates(map[string]any{
		"state": state, "last_error": nakMessage,
	}).Error; err != nil {
		return err
	}
	s.notifyOtaOutcome(deviceID, state, existing.Version)
	return nil
}

func (s *Store) notifyOtaOutcome(deviceID uint, state, version string) {
	name := "设备"
	if dev, err := s.DeviceByID(deviceID); err == nil {
		if dev.Name != "" {
			name = dev.Name
		} else {
			name = dev.SN
		}
	}
	switch state {
	case model.OtaStateSuccess:
		_ = s.CreateNotification(model.NotificationTypeOta, model.NotificationLevelInfo,
			"固件升级成功", name+" 已成功升级到 "+version)
	case model.OtaStateFailed, model.OtaStateRolledBack:
		_ = s.CreateNotification(model.NotificationTypeOta, model.NotificationLevelWarning,
			"固件升级失败", name+" 升级到 "+version+" 失败")
	}
}
