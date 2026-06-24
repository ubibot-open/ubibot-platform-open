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

// Package ha publishes device discovery and state messages to a Home
// Assistant MQTT broker.
package ha

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// defaultFields lists the field slots announced as HA entities when a device
// registers, before any user-defined field definitions are configured.
var defaultFields = []Metric{
	{ID: "field1", Name: "Field 1"},
	{ID: "field2", Name: "Field 2"},
	{ID: "field3", Name: "Field 3"},
}

// Metric describes a sensor entity exposed to Home Assistant.
type Metric struct {
	ID    string
	Name  string
	Unit  string
	Class string // device_class
}

// Config holds HA broker connection parameters.
type Config struct {
	Broker          string
	Username        string
	Password        string
	DiscoveryPrefix string
	ClientID        string
}

// Client connects to a Home Assistant MQTT broker and publishes device
// discovery configs and state values.
type Client struct {
	cfg       Config
	client    paho.Client
	published sync.Map // key "deviceID:metric" -> published bool
}

// New creates an HA client. Returns nil (no-op) semantics are handled by
// the caller checking cfg.Enabled before constructing.
func New(cfg Config) *Client {
	return &Client{cfg: cfg}
}

// Connect establishes the connection to the HA broker.
func (c *Client) Connect() error {
	opts := paho.NewClientOptions().
		AddBroker(c.cfg.Broker).
		SetClientID(c.cfg.ClientID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second)
	if c.cfg.Username != "" {
		opts.SetUsername(c.cfg.Username)
		opts.SetPassword(c.cfg.Password)
	}

	c.client = paho.NewClient(opts)
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("connect to HA broker: %w", token.Error())
	}
	log.Printf("ha client connected to %s", c.cfg.Broker)
	return nil
}

// Disconnect closes the HA broker connection.
func (c *Client) Disconnect() {
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(250)
	}
}

// PublishDeviceDiscovery sends HA discovery messages for the default field
// slots of a newly registered device.
func (c *Client) PublishDeviceDiscovery(deviceID, deviceName string) {
	for _, m := range defaultFields {
		c.publishDiscovery(deviceID, deviceName, m)
	}
}

// PublishState pushes a single field value to HA's state topic.
// If discovery for the field has not been published yet, it is published
// on demand so new fields automatically appear in HA.
func (c *Client) PublishState(deviceID, deviceName, fieldKey, value string) {
	if c.client == nil || !c.client.IsConnected() {
		return
	}
	if _, ok := c.published.Load(discoveryKey(deviceID, fieldKey)); !ok {
		c.publishDiscovery(deviceID, deviceName, Metric{ID: fieldKey, Name: fieldKey})
	}
	topic := stateTopic(c.cfg.DiscoveryPrefix, deviceID, fieldKey)
	body, _ := json.Marshal(map[string]any{"value": value})
	token := c.client.Publish(topic, 0, false, string(body))
	token.Wait()
}

func (c *Client) publishDiscovery(deviceID, deviceName string, m Metric) {
	if c.client == nil || !c.client.IsConnected() {
		return
	}
	if _, loaded := c.published.LoadOrStore(discoveryKey(deviceID, m.ID), true); loaded {
		return
	}
	payload := map[string]any{
		"name":                     fmt.Sprintf("%s %s", deviceName, m.Name),
		"state_topic":              stateTopic(c.cfg.DiscoveryPrefix, deviceID, m.ID),
		"unique_id":                fmt.Sprintf("%s_%s", deviceID, m.ID),
		"value_template":           "{{ value_json.value }}",
		"json_attributes_template": "{{ value_json | tojson }}",
		"device": map[string]any{
			"identifiers":  []string{deviceID},
			"name":         fmt.Sprintf("UbiBot %s", deviceName),
			"model":        "UbiBot",
			"manufacturer": "UbiBot Open",
		},
	}
	if m.Unit != "" {
		payload["unit_of_measurement"] = m.Unit
	}
	if m.Class != "" {
		payload["device_class"] = m.Class
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ha discovery marshal failed: %v", err)
		return
	}
	topic := configTopic(c.cfg.DiscoveryPrefix, deviceID, m.ID)
	token := c.client.Publish(topic, 0, true, body)
	token.Wait()
}

// PublishAlert pushes an alert event to a dedicated topic that HA or other
// subscribers can listen to for automations.
func (c *Client) PublishAlert(deviceID string, alert map[string]any) {
	if c.client == nil || !c.client.IsConnected() {
		return
	}
	body, err := json.Marshal(alert)
	if err != nil {
		log.Printf("ha alert marshal failed: %v", err)
		return
	}
	topic := fmt.Sprintf("ubibot/alerts/%s", deviceID)
	c.client.Publish(topic, 0, false, body)
}

func discoveryKey(deviceID, metric string) string {
	return deviceID + ":" + metric
}

func configTopic(prefix, deviceID, metric string) string {
	return fmt.Sprintf("%s/sensor/%s/%s/config", prefix, deviceID, metric)
}

func stateTopic(prefix, deviceID, metric string) string {
	return fmt.Sprintf("%s/sensor/%s/%s/state", prefix, deviceID, metric)
}

