// Package store's scheduled-task half: lets an operator queue a command
// to run on a repeating cadence (or a plain interval) instead of manually
// dispatching it every time. Kept to two schedule kinds — a fixed
// interval, or a daily time of day — rather than a full cron expression,
// which would need a parser dependency this sandbox's restricted network
// can't reliably fetch, and covers the actual common cases (periodic
// health-check commands, nightly maintenance windows).
package store

import (
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/model"
)

// ScheduledTaskInput is the operator-facing shape for creating/updating a
// scheduled task.
type ScheduledTaskInput struct {
	Name            string
	DeviceID        uint // 0 = every enabled device
	CmdType         string
	CmdArgs         map[string]interface{}
	ScheduleType    string // model.ScheduleTypeInterval | model.ScheduleTypeDaily
	IntervalSeconds int
	DailyAtMinute   int
	Enabled         bool
}

func marshalCmdArgs(args map[string]interface{}) (string, error) {
	if args == nil {
		return "", nil
	}
	b, err := json.Marshal(args)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// computeNextRun works out when a task next fires, counting forward from
// "from" — used both at creation and after every run.
func computeNextRun(t *model.ScheduledTask, from time.Time) time.Time {
	if t.ScheduleType == model.ScheduleTypeDaily {
		next := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location()).
			Add(time.Duration(t.DailyAtMinute) * time.Minute)
		for !next.After(from) {
			next = next.Add(24 * time.Hour)
		}
		return next
	}
	interval := time.Duration(t.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = time.Hour
	}
	return from.Add(interval)
}

func (s *Store) CreateScheduledTask(in ScheduledTaskInput) (*model.ScheduledTask, error) {
	argsJSON, err := marshalCmdArgs(in.CmdArgs)
	if err != nil {
		return nil, err
	}
	t := &model.ScheduledTask{
		Name: in.Name, DeviceID: in.DeviceID, CmdType: in.CmdType, CmdArgs: argsJSON,
		ScheduleType: in.ScheduleType, IntervalSeconds: in.IntervalSeconds, DailyAtMinute: in.DailyAtMinute,
		Enabled: in.Enabled,
	}
	t.NextRunAt = computeNextRun(t, time.Now())
	if err := s.db.Create(t).Error; err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Store) ListScheduledTasks() ([]model.ScheduledTask, error) {
	var rows []model.ScheduledTask
	err := s.db.Order("id asc").Find(&rows).Error
	return rows, err
}

func (s *Store) ScheduledTaskByID(id uint) (*model.ScheduledTask, error) {
	var t model.ScheduledTask
	if err := s.db.First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

// UpdateScheduledTask replaces a task's definition and recomputes its
// next run time from now — editing a schedule always re-arms it rather
// than trying to preserve a stale NextRunAt computed under the old rule.
func (s *Store) UpdateScheduledTask(id uint, in ScheduledTaskInput) error {
	argsJSON, err := marshalCmdArgs(in.CmdArgs)
	if err != nil {
		return err
	}
	t := &model.ScheduledTask{
		Name: in.Name, DeviceID: in.DeviceID, CmdType: in.CmdType, CmdArgs: argsJSON,
		ScheduleType: in.ScheduleType, IntervalSeconds: in.IntervalSeconds, DailyAtMinute: in.DailyAtMinute,
	}
	next := computeNextRun(t, time.Now())
	return s.db.Model(&model.ScheduledTask{}).Where("id = ?", id).Updates(map[string]any{
		"name": in.Name, "device_id": in.DeviceID, "cmd_type": in.CmdType, "cmd_args": argsJSON,
		"schedule_type": in.ScheduleType, "interval_seconds": in.IntervalSeconds, "daily_at_minute": in.DailyAtMinute,
		"enabled": in.Enabled, "next_run_at": next,
	}).Error
}

func (s *Store) SetScheduledTaskEnabled(id uint, enabled bool) error {
	return s.db.Model(&model.ScheduledTask{}).Where("id = ?", id).Update("enabled", enabled).Error
}

func (s *Store) DeleteScheduledTask(id uint) error {
	return s.db.Delete(&model.ScheduledTask{}, id).Error
}

// RunDueScheduledTasks is called periodically by cmd/server's ticker: for
// every enabled task whose NextRunAt has passed, queue the command against
// its target device(s) and compute the next run time.
func (s *Store) RunDueScheduledTasks(now time.Time) error {
	var tasks []model.ScheduledTask
	if err := s.db.Where("enabled = ? AND next_run_at <= ?", true, now).Find(&tasks).Error; err != nil {
		return err
	}
	for i := range tasks {
		if err := s.runScheduledTask(&tasks[i], now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) runScheduledTask(t *model.ScheduledTask, now time.Time) error {
	var args map[string]interface{}
	if t.CmdArgs != "" {
		_ = json.Unmarshal([]byte(t.CmdArgs), &args)
	}

	targetIDs := []uint{t.DeviceID}
	if t.DeviceID == 0 {
		devices, _, err := s.ListDevices(1, 10000)
		if err != nil {
			return err
		}
		targetIDs = nil
		for _, d := range devices {
			if d.Status == model.DeviceStatusEnabled {
				targetIDs = append(targetIDs, d.ID)
			}
		}
	}
	for _, id := range targetIDs {
		if _, err := s.QueueCommand(id, t.CmdType, args); err != nil {
			return err
		}
	}

	next := computeNextRun(t, now)
	return s.db.Model(&model.ScheduledTask{}).Where("id = ?", t.ID).Updates(map[string]any{
		"last_run_at": now, "next_run_at": next,
	}).Error
}
