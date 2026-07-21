// Package store is an in-memory backing store for devices, their
// configuration, queued commands and received telemetry. A real deployment
// would back this with a database; an in-memory map is enough to exercise
// and test the protocol handling in internal/api, which is this package's
// only job.
package store

import (
	"fmt"
	"sync"

	"github.com/ubibot/ubibot-platform-open/internal/protocol"
)

// Device is the factory-provisioned identity triple. Did (used in
// /api/v1/data/report) is treated as equal to SN: the protocol doc has no
// separate device-registration step that would mint a distinct id.
type Device struct {
	PID    string
	SN     string
	Secret string
}

// DefaultConfig is handed to a device that has never had its configuration
// changed.
var DefaultConfig = protocol.Config{CI: 30, UI: 600}

// maxPendingCmdPerResponse caps how many queued commands are attached to a
// single report response. Not specified by the protocol doc, but without
// some bound an unbounded backlog could push the response past the 1024B
// budget the rest of the protocol is designed around; overflow just waits
// for the next report cycle instead of being dropped.
const maxPendingCmdPerResponse = 8

type deviceState struct {
	cfg            protocol.Config
	cfgVersion     int
	lastSentCfgVer int
	pending        []protocol.CmdItem
	nextCmdSeq     int
}

// Store aggregates all server-side state needed to serve the device-facing
// endpoints. A single mutex is enough at this scale; it is not a
// performance-sensitive path.
type Store struct {
	mu sync.Mutex

	devices map[string]Device // key: sn
	states  map[string]*deviceState
	seen    map[string]struct{} // dedup key: "did|ts"
}

func New() *Store {
	return &Store{
		devices: make(map[string]Device),
		states:  make(map[string]*deviceState),
		seen:    make(map[string]struct{}),
	}
}

// RegisterDevice provisions a device triple. Intended for test/demo seeding.
func (s *Store) RegisterDevice(d Device) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.devices[d.SN] = d
}

// Device looks up a device by serial number.
func (s *Store) Device(sn string) (Device, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.devices[sn]
	return d, ok
}

func (s *Store) state(did string) *deviceState {
	st, ok := s.states[did]
	if !ok {
		st = &deviceState{cfg: DefaultConfig}
		s.states[did] = st
	}
	return st
}

// SetConfig updates a device's desired configuration, marking it dirty so
// the next report response includes it.
func (s *Store) SetConfig(did string, cfg protocol.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state(did)
	st.cfg = cfg
	st.cfgVersion++
}

// QueueCommand appends a command for did, assigning it a unique id.
func (s *Store) QueueCommand(did, tp string, args map[string]interface{}) protocol.CmdItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state(did)
	st.nextCmdSeq++
	item := protocol.CmdItem{ID: fmt.Sprintf("c%d", st.nextCmdSeq), Tp: tp, A: args}
	st.pending = append(st.pending, item)
	return item
}

// AckCommands removes acknowledged command ids from did's pending queue.
func (s *Store) AckCommands(did string, ids []string) {
	if len(ids) == 0 {
		return
	}
	acked := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		acked[id] = struct{}{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state(did)
	kept := st.pending[:0]
	for _, c := range st.pending {
		if _, done := acked[c.ID]; !done {
			kept = append(kept, c)
		}
	}
	st.pending = kept
}

// ReportOutcome is what ProcessReport hands back for the handler to render
// into a protocol.ReportResponse.
type ReportOutcome struct {
	Cfg *protocol.Config
	Cmd []protocol.CmdItem
}

// ProcessReport records the (deduplicated) records in recs, applies acks,
// and returns the config (only if changed since last delivery) and up to
// maxPendingCmdPerResponse queued commands to send back.
func (s *Store) ProcessReport(did string, recs []protocol.Record, ack []string) ReportOutcome {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, r := range recs {
		key := fmt.Sprintf("%s|%d", did, r.Ts)
		s.seen[key] = struct{}{}
	}

	st := s.state(did)

	if len(ack) > 0 {
		acked := make(map[string]struct{}, len(ack))
		for _, id := range ack {
			acked[id] = struct{}{}
		}
		kept := st.pending[:0]
		for _, c := range st.pending {
			if _, done := acked[c.ID]; !done {
				kept = append(kept, c)
			}
		}
		st.pending = kept
	}

	var out ReportOutcome
	if st.cfgVersion != st.lastSentCfgVer {
		cfg := st.cfg
		out.Cfg = &cfg
		st.lastSentCfgVer = st.cfgVersion
	}
	if n := len(st.pending); n > 0 {
		if n > maxPendingCmdPerResponse {
			n = maxPendingCmdPerResponse
		}
		out.Cmd = append(out.Cmd, st.pending[:n]...)
	}
	return out
}

// Seen reports whether (did, ts) has already been recorded — exposed for
// tests that check dedup behaviour.
func (s *Store) Seen(did string, ts int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.seen[fmt.Sprintf("%s|%d", did, ts)]
	return ok
}
