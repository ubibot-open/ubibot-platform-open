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

// Package coap provides a lightweight CoAP server for IoT device connectivity.
// It handles two resource paths:
//
//   - PUT /telemetry  – device uploads a protocol.TelemetryPayload
//   - GET /config     – device fetches its protocol.ConfigPayload
package coap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	coap "github.com/plgd-dev/go-coap/v3"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/mux"
	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/models"
	"github.com/ubibot/ubibot-platform-open/internal/protocol"
)

// DeviceEvents is called when a device sends telemetry over CoAP.
type DeviceEvents interface {
	OnTelemetry(deviceID string, payload []byte)
}

// Server wraps the CoAP UDP listener.
type Server struct {
	port   int
	db     *gorm.DB
	events DeviceEvents
}

// New creates a CoAP Server listening on the given UDP port (default 5683).
func New(port int, db *gorm.DB, events DeviceEvents) *Server {
	return &Server{port: port, db: db, events: events}
}

// Start begins serving CoAP requests in the background and returns
// immediately. Call the returned stop function to shut down.
func (s *Server) Start(ctx context.Context) error {
	r := mux.NewRouter()
	r.Handle("/telemetry", mux.HandlerFunc(s.handleTelemetry))
	r.Handle("/config", mux.HandlerFunc(s.handleConfig))

	addr := fmt.Sprintf(":%d", s.port)
	go func() {
		log.Printf("coap server listening on udp%s", addr)
		if err := coap.ListenAndServe("udp", addr, r); err != nil {
			// ListenAndServe only returns on fatal error; the context-based
			// shutdown path sends the process SIGINT which closes the listener.
			log.Printf("coap server stopped: %v", err)
		}
	}()

	return nil
}

// handleTelemetry processes PUT /telemetry from a device.
//
// Expected content-format: application/json (CoAP content-format 50).
// The payload must be a valid protocol.TelemetryPayload with device_id and
// token fields for authentication.
func (s *Server) handleTelemetry(w mux.ResponseWriter, r *mux.Message) {
	if r.Code() != codes.PUT && r.Code() != codes.POST {
		_ = w.SetResponse(codes.MethodNotAllowed, message.TextPlain, nil)
		return
	}

	body, err := r.Message.ReadBody()
	if err != nil {
		_ = w.SetResponse(codes.BadRequest, message.TextPlain, nil)
		return
	}

	p, err := protocol.ParseTelemetry(body)
	if err != nil {
		log.Printf("coap telemetry parse error: %v", err)
		_ = w.SetResponse(codes.BadRequest, message.TextPlain, nil)
		return
	}

	if !s.authenticate(p.DeviceID, p.Token) {
		_ = w.SetResponse(codes.Unauthorized, message.TextPlain, nil)
		return
	}

	s.events.OnTelemetry(p.DeviceID, body)

	// Return the current config as the CoAP response payload so devices can
	// update their parameters in a single round-trip.
	cfg := s.configForDevice(p.DeviceID)
	data, _ := json.Marshal(cfg)
	_ = w.SetResponse(codes.Changed, message.AppJSON, bytes.NewReader(data))
}

// handleConfig processes GET /config from a device.
//
// The device must supply ?device_id=<id>&token=<token> query parameters.
func (s *Server) handleConfig(w mux.ResponseWriter, r *mux.Message) {
	if r.Code() != codes.GET {
		_ = w.SetResponse(codes.MethodNotAllowed, message.TextPlain, nil)
		return
	}

	queries, err := r.Options().Queries()
	if err != nil || len(queries) == 0 {
		_ = w.SetResponse(codes.BadRequest, message.TextPlain, nil)
		return
	}

	params := parseQuerySlice(queries)
	deviceID := params["device_id"]
	token := params["token"]

	if deviceID == "" || !s.authenticate(deviceID, token) {
		_ = w.SetResponse(codes.Unauthorized, message.TextPlain, nil)
		return
	}

	cfg := s.configForDevice(deviceID)
	data, _ := json.Marshal(cfg)
	_ = w.SetResponse(codes.Content, message.AppJSON, bytes.NewReader(data))
}

// authenticate verifies that token matches the device's stored token.
func (s *Server) authenticate(deviceID, token string) bool {
	if deviceID == "" || token == "" {
		return false
	}
	var d models.Device
	if err := s.db.Where("device_id = ?", deviceID).First(&d).Error; err != nil {
		return false
	}
	return d.Token == token
}

// configForDevice returns a ConfigPayload for the given device.
func (s *Server) configForDevice(deviceID string) protocol.ConfigPayload {
	var dc models.DeviceConfig
	if err := s.db.Where("device_id = ?", deviceID).First(&dc).Error; err != nil {
		return protocol.ConfigPayload{
			CollectInterval: 30,
			UploadInterval:  60,
			ServerTime:      time.Now().Unix(),
		}
	}

	var enabled []string
	if dc.SensorsEnabled != "" {
		_ = json.Unmarshal([]byte(dc.SensorsEnabled), &enabled)
	}

	return protocol.ConfigPayload{
		CollectInterval: dc.CollectInterval,
		UploadInterval:  dc.UploadInterval,
		SensorsEnabled:  enabled,
		ServerTime:      time.Now().Unix(),
	}
}

// parseQuerySlice converts a []string of "key=value" pairs into a map.
func parseQuerySlice(queries []string) map[string]string {
	m := make(map[string]string, len(queries))
	for _, q := range queries {
		for i := 0; i < len(q); i++ {
			if q[i] == '=' {
				m[q[:i]] = q[i+1:]
				break
			}
		}
	}
	return m
}

