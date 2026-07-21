package store

import (
	"encoding/json"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/ubibot/ubibot-platform-open/internal/model"
)

// ProbeInput is the operator-facing shape for defining a custom probe
// (protocol §7.2). Params carries whatever fields the chosen iface/proto
// needs (addr/fc/reg/cnt/dtype/byte_order/scale/offset/ci/timeout/retry) —
// see docs/UbiBot开放平台硬件通信协议.md §7.2 for the field meanings.
type ProbeInput struct {
	Pid    string
	Key    string
	Iface  string
	Proto  string
	Params map[string]any
}

// UpsertProbe queues a set_probe(op=upsert) command and records the
// desired probe definition as "pending" until that command is acked or
// nacked (see applyProbeCommandOutcome, wired in from ProcessReport).
func (s *Store) UpsertProbe(deviceID uint, in ProbeInput) (*model.DeviceProbe, *model.DeviceCommand, error) {
	paramsJSON, err := json.Marshal(in.Params)
	if err != nil {
		return nil, nil, err
	}

	wireProbe := map[string]any{"pid": in.Pid, "key": in.Key, "iface": in.Iface, "proto": in.Proto}
	for k, v := range in.Params {
		wireProbe[k] = v
	}
	cmd, err := s.QueueCommand(deviceID, "set_probe", map[string]any{
		"op":     "upsert",
		"probes": []map[string]any{wireProbe},
	})
	if err != nil {
		return nil, nil, err
	}

	probe := &model.DeviceProbe{
		DeviceID:      deviceID,
		Pid:           in.Pid,
		Key:           in.Key,
		Iface:         in.Iface,
		Proto:         in.Proto,
		Params:        string(paramsJSON),
		Status:        model.ProbeStatusPending,
		LastCommandID: cmd.CmdID,
	}
	err = s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "device_id"}, {Name: "pid"}},
		DoUpdates: clause.AssignmentColumns(
			[]string{"key", "iface", "proto", "params", "status", "last_command_id", "last_error"}),
	}).Create(probe).Error
	if err != nil {
		return nil, nil, err
	}
	return probe, cmd, nil
}

// RemoveProbe queues a set_probe(op=remove) command and marks the probe
// "removing" — it's deleted from device_probes only once that command is
// acked (applyProbeCommandOutcome), so a failed removal leaves the row in
// place with the nak reason instead of silently vanishing.
func (s *Store) RemoveProbe(deviceID uint, pid string) (*model.DeviceCommand, error) {
	var probe model.DeviceProbe
	if err := s.db.Where("device_id = ? AND pid = ?", deviceID, pid).First(&probe).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	cmd, err := s.QueueCommand(deviceID, "set_probe", map[string]any{
		"op":     "remove",
		"probes": []map[string]any{{"pid": pid}},
	})
	if err != nil {
		return nil, err
	}

	err = s.db.Model(&probe).Updates(map[string]any{
		"status":          model.ProbeStatusRemoving,
		"last_command_id": cmd.CmdID,
	}).Error
	if err != nil {
		return nil, err
	}
	return cmd, nil
}

// ListProbes returns every probe configured for deviceID.
func (s *Store) ListProbes(deviceID uint) ([]model.DeviceProbe, error) {
	var rows []model.DeviceProbe
	err := s.db.Where("device_id = ?", deviceID).Order("id asc").Find(&rows).Error
	return rows, err
}

// applyProbeCommandOutcome reacts to a set_probe command being acked or
// nacked: an acked removal deletes the row, an acked upsert flips it to
// applied, and any nak records the device's reported reason instead of
// silently leaving the probe stuck "pending".
func (s *Store) applyProbeCommandOutcome(deviceID uint, cmdID string, acked bool, nakMessage string) error {
	var probe model.DeviceProbe
	err := s.db.Where("device_id = ? AND last_command_id = ?", deviceID, cmdID).First(&probe).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil // this command wasn't a set_probe, or wasn't tied to a probe row
	}
	if err != nil {
		return err
	}

	if !acked {
		return s.db.Model(&probe).Updates(map[string]any{
			"status":     model.ProbeStatusFailed,
			"last_error": nakMessage,
		}).Error
	}

	if probe.Status == model.ProbeStatusRemoving {
		return s.db.Delete(&probe).Error
	}
	return s.db.Model(&probe).Updates(map[string]any{
		"status":     model.ProbeStatusApplied,
		"last_error": "",
	}).Error
}
