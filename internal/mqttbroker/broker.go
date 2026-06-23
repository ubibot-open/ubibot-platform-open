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

// Package mqttbroker embeds an MQTT server for device connectivity.
package mqttbroker

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

// TelemetryTopicPrefix is the topic prefix devices publish telemetry to.
// Full topic format: ubibot/{device_id}/telemetry
const TelemetryTopicPrefix = "ubibot/"

// DeviceEvents is the callback interface the broker uses to notify the
// rest of the platform about device activity.
type DeviceEvents interface {
	OnTelemetry(deviceID string, payload []byte)
	OnConnect(clientID string)
	OnDisconnect(clientID string)
}

// Broker wraps the embedded mochi-mqtt server.
type Broker struct {
	server *mqtt.Server
	port   int
	events DeviceEvents
}

// New creates a broker listening on the given port. The events receiver
// is invoked for every telemetry publish and connect/disconnect.
func New(port int, events DeviceEvents) *Broker {
	b := &Broker{
		server: mqtt.New(&mqtt.Options{InlineClient: true}),
		port:   port,
		events: events,
	}
	return b
}

// Start registers hooks and listeners and begins serving.
func (b *Broker) Start() error {
	// Allow all device connections. Authentication can be added later.
	if err := b.server.AddHook(new(auth.AllowHook), nil); err != nil {
		return fmt.Errorf("add auth hook: %w", err)
	}

	if err := b.server.AddHook(&telemetryHook{events: b.events}, nil); err != nil {
		return fmt.Errorf("add telemetry hook: %w", err)
	}

	addr := fmt.Sprintf(":%d", b.port)
	tcp := listeners.NewTCP(listeners.Config{ID: "tcp-main", Address: addr})
	if err := b.server.AddListener(tcp); err != nil {
		return fmt.Errorf("add tcp listener: %w", err)
	}

	go func() {
		if err := b.server.Serve(); err != nil {
			log.Printf("mqtt broker serve error: %v", err)
		}
	}()

	log.Printf("mqtt broker listening on :%d", b.port)
	return nil
}

// Close stops the broker.
func (b *Broker) Close() error {
	return b.server.Close()
}

// Publish allows the platform to push a message down to a device topic.
func (b *Broker) Publish(topic string, payload []byte, retain bool) error {
	return b.server.Publish(topic, payload, retain, 0)
}

// telemetryHook intercepts device publishes and connection lifecycle events.
type telemetryHook struct {
	mqtt.HookBase
	events DeviceEvents
}

func (h *telemetryHook) ID() string { return "ubibot-telemetry" }

func (h *telemetryHook) Init(config any) error {
	return nil
}

// Provides declares which hook events this hook handles. Without this,
// mochi-mqtt skips the corresponding callbacks.
func (h *telemetryHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnect,
		mqtt.OnDisconnect,
		mqtt.OnPublish,
	}, []byte{b})
}

func (h *telemetryHook) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
	if h.events != nil {
		h.events.OnConnect(cl.ID)
	}
	return nil
}

func (h *telemetryHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	if h.events != nil {
		h.events.OnDisconnect(cl.ID)
	}
}

func (h *telemetryHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	if h.events != nil && strings.HasPrefix(pk.TopicName, TelemetryTopicPrefix) {
		if deviceID := extractDeviceID(pk.TopicName); deviceID != "" {
			h.events.OnTelemetry(deviceID, pk.Payload)
		}
	}
	return pk, nil
}

// extractDeviceID parses "ubibot/{device_id}/telemetry" and returns device_id.
func extractDeviceID(topic string) string {
	const suffix = "/telemetry"
	if !strings.HasPrefix(topic, TelemetryTopicPrefix) || !strings.HasSuffix(topic, suffix) {
		return ""
	}
	middle := topic[len(TelemetryTopicPrefix) : len(topic)-len(suffix)]
	if middle == "" || strings.Contains(middle, "/") {
		return ""
	}
	return middle
}
