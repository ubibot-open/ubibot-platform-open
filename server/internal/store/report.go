package store

import (
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/protocol"
)

// ProcessReport is the single entry point for POST /data/report's business
// logic (docs §4): persist the (deduplicated) payloads, mark the device as
// just-seen, and evaluate threshold alert rules against the freshest
// values. There is no cfg/cmd channel to reply with anymore — the caller
// just acks.
func (s *Store) ProcessReport(dev *model.Device, payloads []protocol.Payload, now time.Time) error {
	if err := s.SaveRecords(dev.ID, payloads); err != nil {
		return err
	}
	if err := s.TouchLastSeen(dev.ID, now); err != nil {
		return err
	}
	if err := s.evaluateThresholdRules(dev.ID, payloads); err != nil {
		return err
	}
	return nil
}
