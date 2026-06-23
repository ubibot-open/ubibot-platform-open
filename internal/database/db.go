package database

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/ubibot/ubibot-platform-open/internal/models"
)

// Open initializes a SQLite database connection using the pure-Go
// modernc.org/sqlite driver (no CGO required) and runs auto-migration.
func Open(dbPath string) (*gorm.DB, error) {
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.AutoMigrate(
		&models.Device{},
		&models.User{},
		&models.Rule{},
		&models.Telemetry{},
		&models.Alert{},
	); err != nil {
		return nil, fmt.Errorf("auto-migrate: %w", err)
	}

	return db, nil
}

// CleanupOldTelemetry deletes telemetry rows older than retentionDays.
// Intended to be called by a periodic cleanup job.
func CleanupOldTelemetry(db *gorm.DB, retentionDays int) (int64, error) {
	cutoff := fmt.Sprintf("datetime('now','-%d days')", retentionDays)
	result := db.Where("timestamp < " + cutoff).Delete(&models.Telemetry{})
	return result.RowsAffected, result.Error
}
