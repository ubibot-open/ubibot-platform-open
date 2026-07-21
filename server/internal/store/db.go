// Package store is the persistence layer: device identity, issued tokens,
// telemetry history, and the command queue, all backed by SQLite through
// GORM. Everything the device-facing and admin-facing handlers need to
// read or write goes through the Store methods in this package — handlers
// never touch *gorm.DB directly.
package store

import (
	"fmt"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/ubibot/ubibot-platform-open/internal/model"
)

// Store wraps the database handle. The mutex only guards the in-memory
// nonce map (see nonce.go in internal/auth, which is intentionally kept
// separate and non-persistent); everything durable goes through GORM,
// which handles its own concurrency.
type Store struct {
	db *gorm.DB
	mu sync.Mutex
}

// Open opens (creating if needed) the SQLite database at path and runs
// AutoMigrate for every model this package owns. path may be ":memory:"
// for tests.
func Open(path string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.AutoMigrate(
		&model.Device{},
		&model.DeviceToken{},
		&model.DeviceRecord{},
		&model.DeviceCommand{},
		&model.DeviceProbe{},
		&model.AlertRule{},
		&model.AlertEvent{},
		&model.Role{},
		&model.AdminUser{},
		&model.AdminSession{},
		&model.AuditLog{},
		&model.Firmware{},
		&model.DeviceOTA{},
		&model.Notification{},
		&model.ScheduledTask{},
		&model.ApiKey{},
		&model.FileAsset{},
		&model.DictEntry{},
		&model.SystemParam{},
	); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}
