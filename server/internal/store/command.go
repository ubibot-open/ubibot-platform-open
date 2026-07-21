package store

import (
	"encoding/json"
	"fmt"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/protocol"
)

// maxPendingCmdPerReport caps how many queued commands are attached to a
// single report/poll response. Not specified by the protocol doc, but
// without some bound an unbounded backlog could push the response past
// the byte budget the rest of the protocol is designed around; overflow
// just waits for the next report cycle instead of being dropped.
const maxPendingCmdPerReport = 8

// QueueCommand appends a command for deviceID. The assigned row id also
// becomes the wire-protocol CmdID ("c<id>") — globally unique, and stable
// once assigned, which is all §7 requires of it.
func (s *Store) QueueCommand(deviceID uint, tp string, args map[string]interface{}) (*model.DeviceCommand, error) {
	var argsJSON string
	if args != nil {
		b, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		argsJSON = string(b)
	}

	cmd := &model.DeviceCommand{
		DeviceID: deviceID,
		Type:     tp,
		Args:     argsJSON,
		Status:   model.CommandStatusPending,
	}
	if err := s.db.Create(cmd).Error; err != nil {
		return nil, err
	}
	cmd.CmdID = fmt.Sprintf("c%d", cmd.ID)
	if err := s.db.Model(cmd).Update("cmd_id", cmd.CmdID).Error; err != nil {
		return nil, err
	}
	return cmd, nil
}

// PendingCommands returns up to maxPendingCmdPerReport still-pending
// commands for deviceID, oldest first, rendered as wire-protocol CmdItems.
func (s *Store) PendingCommands(deviceID uint) ([]protocol.CmdItem, error) {
	var rows []model.DeviceCommand
	err := s.db.Where("device_id = ? AND status = ?", deviceID, model.CommandStatusPending).
		Order("id asc").
		Limit(maxPendingCmdPerReport).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	items := make([]protocol.CmdItem, 0, len(rows))
	for _, r := range rows {
		item := protocol.CmdItem{ID: r.CmdID, Tp: r.Type}
		if r.Args != "" {
			var a map[string]interface{}
			if err := json.Unmarshal([]byte(r.Args), &a); err == nil {
				item.A = a
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// AckCommands marks the given command ids (for deviceID, currently
// pending) as acked.
func (s *Store) AckCommands(deviceID uint, cmdIDs []string) error {
	if len(cmdIDs) == 0 {
		return nil
	}
	return s.db.Model(&model.DeviceCommand{}).
		Where("device_id = ? AND cmd_id IN ? AND status = ?", deviceID, cmdIDs, model.CommandStatusPending).
		Update("status", model.CommandStatusAcked).Error
}

// NakCommands marks the given commands (for deviceID, currently pending)
// as failed, recording the reason the device reported.
func (s *Store) NakCommands(deviceID uint, naks []protocol.Nak) error {
	for _, n := range naks {
		err := s.db.Model(&model.DeviceCommand{}).
			Where("device_id = ? AND cmd_id = ? AND status = ?", deviceID, n.ID, model.CommandStatusPending).
			Updates(map[string]any{
				"status":      model.CommandStatusNacked,
				"nak_message": n.M,
			}).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// ListCommands returns a device's command history, newest first, for the
// admin device-detail page.
func (s *Store) ListCommands(deviceID uint, page, pageSize int) ([]model.DeviceCommand, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	var total int64
	if err := s.db.Model(&model.DeviceCommand{}).Where("device_id = ?", deviceID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []model.DeviceCommand
	err := s.db.Where("device_id = ?", deviceID).
		Order("id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
