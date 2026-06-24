// Copyright 2026 UbiBot Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package telemetry buffers incoming device readings and flushes them to
// the database in batches.
package telemetry

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/models"
)

// Record is a single field reading queued for batch insertion.
// Value is the raw string received from the device; consumers that need a
// numeric value should parse it with strconv.ParseFloat.
type Record struct {
	DeviceID  string
	Field     string // "field1".."field20"
	Value     string // raw string value
	Timestamp time.Time
}

// Sink is called for each incoming record before it is buffered,
// allowing downstream consumers (rule engine, HA) to react in real time.
type Sink func(Record)

// Buffer accumulates telemetry records in memory and flushes them to SQLite
// in batches, reducing write pressure on the database.
type Buffer struct {
	db        *gorm.DB
	records   chan Record
	batchSize int
	interval  time.Duration
	sink      Sink
}

// NewBuffer creates a telemetry buffer with the given batch size and flush interval.
// sink may be nil; when set it is invoked for every accepted record.
func NewBuffer(db *gorm.DB, batchSize int, interval time.Duration, sink Sink) *Buffer {
	if batchSize <= 0 {
		batchSize = 100
	}
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return &Buffer{
		db:        db,
		records:   make(chan Record, batchSize*4),
		batchSize: batchSize,
		interval:  interval,
		sink:      sink,
	}
}

// Add queues a telemetry record. Non-blocking: drops the record if the
// buffer is full to avoid blocking the MQTT message path.
func (b *Buffer) Add(r Record) {
	select {
	case b.records <- r:
		if b.sink != nil {
			b.sink(r)
		}
	default:
		log.Printf("telemetry buffer full, dropping record device=%s field=%s", r.DeviceID, r.Field)
	}
}

// Run starts the flush loop. Blocks until ctx is cancelled, flushing any
// remaining records on exit.
func (b *Buffer) Run(ctx context.Context) {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	batch := make([]models.Telemetry, 0, b.batchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := b.db.CreateInBatches(&batch, len(batch)).Error; err != nil {
			log.Printf("telemetry batch insert failed: %v", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case r := <-b.records:
			batch = append(batch, models.Telemetry{
				DeviceID:  r.DeviceID,
				Field:     r.Field,
				Value:     r.Value,
				Timestamp: r.Timestamp,
			})
			if len(batch) >= b.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			// Drain remaining queued records before exiting.
			for {
				select {
				case r := <-b.records:
					batch = append(batch, models.Telemetry{
						DeviceID:  r.DeviceID,
						Field:     r.Field,
						Value:     r.Value,
						Timestamp: r.Timestamp,
					})
				default:
					flush()
					return
				}
			}
		}
	}
}
