package store

import (
	"encoding/json"
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/protocol"
)

// ReportOutcome is what ProcessReport hands back for the handler to
// render into a protocol.ReportResponse.
type ReportOutcome struct {
	Cfg *protocol.Config
	Cmd []protocol.CmdItem
}

// ProcessReport is the single entry point for POST /data/report's business
// logic: persist the (deduplicated) records, apply any ack/nak against the
// command queue, mark the device as just-seen, and work out what to hand
// back — the config only if it changed since it was last delivered, plus
// whatever commands are still pending.
func (s *Store) ProcessReport(dev *model.Device, recs []protocol.Record, ack []string, nak []protocol.Nak, now time.Time) (ReportOutcome, error) {
	if err := s.SaveRecords(dev.ID, recs); err != nil {
		return ReportOutcome{}, err
	}
	if err := s.applyAckNak(dev.ID, ack, nak); err != nil {
		return ReportOutcome{}, err
	}
	if err := s.TouchLastSeen(dev.ID, now); err != nil {
		return ReportOutcome{}, err
	}
	if err := s.evaluateThresholdRules(dev.ID, recs); err != nil {
		return ReportOutcome{}, err
	}

	var out ReportOutcome

	if dev.CfgVersion != dev.LastSentCfgVersion {
		var fe []string
		if dev.FE != "" {
			_ = json.Unmarshal([]byte(dev.FE), &fe)
		}
		out.Cfg = &protocol.Config{CI: dev.CI, UI: dev.UI, FE: fe}
		if err := s.db.Model(&model.Device{}).Where("id = ?", dev.ID).
			Update("last_sent_cfg_version", dev.CfgVersion).Error; err != nil {
			return ReportOutcome{}, err
		}
	}

	cmds, err := s.PendingCommands(dev.ID)
	if err != nil {
		return ReportOutcome{}, err
	}
	out.Cmd = cmds

	return out, nil
}

// applyAckNak updates the command queue for the given ack/nak ids and,
// for any of them that were a set_probe command, propagates the outcome
// onto the corresponding device_probes row (see applyProbeCommandOutcome
// in probe.go) — ack/nak is otherwise generic to every command type.
func (s *Store) applyAckNak(deviceID uint, ack []string, nak []protocol.Nak) error {
	if err := s.AckCommands(deviceID, ack); err != nil {
		return err
	}
	if err := s.NakCommands(deviceID, nak); err != nil {
		return err
	}

	nakIDs := make([]string, len(nak))
	nakMsg := make(map[string]string, len(nak))
	for i, n := range nak {
		nakIDs[i] = n.ID
		nakMsg[n.ID] = n.M
	}

	allIDs := append(append([]string{}, ack...), nakIDs...)
	types, err := s.CommandTypesByIDs(deviceID, allIDs)
	if err != nil {
		return err
	}

	for _, id := range ack {
		if types[id] == "set_probe" {
			if err := s.applyProbeCommandOutcome(deviceID, id, true, ""); err != nil {
				return err
			}
		}
	}
	for _, id := range nakIDs {
		if types[id] == "set_probe" {
			if err := s.applyProbeCommandOutcome(deviceID, id, false, nakMsg[id]); err != nil {
				return err
			}
		}
	}
	return nil
}
