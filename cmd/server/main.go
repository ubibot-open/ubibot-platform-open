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

// Package main provides the entry point for the UbiBot platform server.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/api"
	"github.com/ubibot/ubibot-platform-open/internal/coap"
	"github.com/ubibot/ubibot-platform-open/internal/config"
	"github.com/ubibot/ubibot-platform-open/internal/database"
	"github.com/ubibot/ubibot-platform-open/internal/ha"
	"github.com/ubibot/ubibot-platform-open/internal/mqttbroker"
	"github.com/ubibot/ubibot-platform-open/internal/protocol"
	"github.com/ubibot/ubibot-platform-open/internal/rule"
	"github.com/ubibot/ubibot-platform-open/internal/telemetry"
	"gorm.io/gorm"
)

// coordinator wires MQTT device events to the telemetry buffer, rule engine,
// HA client and device status updates. Implements mqttbroker.DeviceEvents.
type coordinator struct {
	buffer    *telemetry.Buffer
	haClient  *ha.Client
	deviceAPI *api.DeviceAPI
}

// OnTelemetry handles an uplink telemetry payload from any transport.
// It parses the TelemetryPayload, fans each DataPoint's field values into
// the buffer (→ rule engine + DB), and forwards them to Home Assistant.
func (co *coordinator) OnTelemetry(deviceID string, payload []byte) {
	p, err := protocol.ParseTelemetry(payload)
	if err != nil {
		log.Printf("telemetry parse failed device=%s: %v", deviceID, err)
		return
	}

	// Use the payload's device_id when the transport (MQTT) provides it via
	// the topic; the coordinator always receives the authoritative deviceID.
	for _, dp := range p.Data {
		ts := dp.Time()
		for fieldKey, value := range dp.Fields() {
			co.buffer.Add(telemetry.Record{
				DeviceID:  deviceID,
				Field:     fieldKey,
				Value:     value,
				Timestamp: ts,
			})
			if co.haClient != nil {
				co.haClient.PublishState(deviceID, deviceID, fieldKey, value)
			}
		}
	}
	co.deviceAPI.UpdateStatus(deviceID, true)
}

func (co *coordinator) OnConnect(clientID string) {
	co.deviceAPI.UpdateStatus(clientID, true)
}

func (co *coordinator) OnDisconnect(clientID string) {
	co.deviceAPI.UpdateStatus(clientID, false)
}

func main() {
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	// Rule engine (loads enabled rules into memory).
	engine := rule.New(db, nil)

	// Telemetry buffer. The sink fans records out to the rule engine.
	buffer := telemetry.NewBuffer(db, cfg.Telemetry.BatchSize, cfg.Telemetry.FlushInterval, engine.Match)

	// Home Assistant client (optional).
	var haClient *ha.Client
	if cfg.HomeAssistant.Enabled {
		haClient = ha.New(ha.Config{
			Broker:          cfg.HomeAssistant.Broker,
			Username:        cfg.HomeAssistant.Username,
			Password:        cfg.HomeAssistant.Password,
			DiscoveryPrefix: cfg.HomeAssistant.DiscoveryPrefix,
			ClientID:        cfg.HomeAssistant.ClientID,
		})
		if err := haClient.Connect(); err != nil {
			log.Printf("ha client disabled: %v", err)
			haClient = nil
		}
	}
	engine.SetNotifier(haClient) // wire alert notifications (nil-safe)

	// Coordinator bridges transport events to the rest of the platform.
	deviceAPI := api.NewDeviceAPI(db, haClient)
	co := &coordinator{
		buffer:    buffer,
		haClient:  haClient,
		deviceAPI: deviceAPI,
	}

	// Embedded MQTT broker.
	broker := mqttbroker.New(cfg.Server.MQTTPort, co)

	// HTTP API (needs coordinator and broker for ingest / config push).
	router := api.NewRouter(db, haClient, engine, buffer, co, broker)

	// CoAP server.
	coapSrv := coap.New(cfg.Server.CoAPPort, db, co)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start telemetry buffer flusher.
	go buffer.Run(ctx)

	// Start HTTP server.
	httpSrv := &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.Server.HTTPPort),
		Handler: router,
	}
	go func() {
		log.Printf("http server listening on %s", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	// Start MQTT broker.
	if err := broker.Start(); err != nil {
		log.Fatalf("start mqtt broker: %v", err)
	}

	// Start CoAP server.
	if err := coapSrv.Start(ctx); err != nil {
		log.Fatalf("start coap server: %v", err)
	}

	// Load rules now that everything is wired.
	if err := engine.Load(); err != nil {
		log.Printf("load rules: %v", err)
	}

	// Telemetry retention cleanup (daily).
	go runCleanup(ctx, db, cfg.Telemetry.RetentionDays)

	log.Printf("ubibot-platform started: http=:%d mqtt=:%d coap=:%d",
		cfg.Server.HTTPPort, cfg.Server.MQTTPort, cfg.Server.CoAPPort)
	<-ctx.Done()
	log.Printf("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	broker.Close()
	if haClient != nil {
		haClient.Disconnect()
	}
	log.Printf("ubibot-platform stopped")
}

func runCleanup(ctx context.Context, db *gorm.DB, retentionDays int) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if n, err := database.CleanupOldTelemetry(db, retentionDays); err != nil {
				log.Printf("telemetry cleanup failed: %v", err)
			} else if n > 0 {
				log.Printf("telemetry cleanup removed %d old rows", n)
			}
		case <-ctx.Done():
			return
		}
	}
}
