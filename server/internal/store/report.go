package store

import (
	"encoding/json"

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
func (s *Store) ProcessReport(dev *model.Device, recs []protocol.Record, ack []string, nak []protocol.Nak) (ReportOutcome, error) {
	if err := s.SaveRecords(dev.ID, recs); err != nil {
		return ReportOutcome{}, err
	}
	if err := s.AckCommands(dev.ID, ack); err != nil {
		return ReportOutcome{}, err
	}
	if err := s.NakCommands(dev.ID, nak); err != nil {
		return ReportOutcome{}, err
	}
	if err := s.TouchLastSeen(dev.ID); err != nil {
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
