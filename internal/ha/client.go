package ha

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// Default metrics published as HA entities when a device registers.
var defaultMetrics = []Metric{
	{ID: "temperature", Name: "Temperature", Unit: "°C", Class: "temperature"},
	{ID: "humidity", Name: "Humidity", Unit: "%", Class: "humidity"},
	{ID: "battery", Name: "Battery", Unit: "%", Class: "battery"},
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

// PublishDeviceDiscovery sends HA discovery messages for the default
// metrics of a device. Called when a device is registered.
func (c *Client) PublishDeviceDiscovery(deviceID, deviceName string) {
	for _, m := range defaultMetrics {
		c.publishDiscovery(deviceID, deviceName, m)
	}
}

// PublishState pushes a single metric value to HA's state topic.
// If discovery for the metric has not been published yet, it is published
// on demand so unknown metrics still appear in HA.
func (c *Client) PublishState(deviceID, deviceName, metric string, value float64) {
	if c.client == nil || !c.client.IsConnected() {
		return
	}
	if _, ok := c.published.Load(discoveryKey(deviceID, metric)); !ok {
		c.publishDiscovery(deviceID, deviceName, Metric{ID: metric, Name: metric, Class: metricClass(metric)})
	}
	stateTopic := stateTopic(c.cfg.DiscoveryPrefix, deviceID, metric)
	token := c.client.Publish(stateTopic, 0, false, formatValue(value))
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

func metricClass(metric string) string {
	switch strings.ToLower(metric) {
	case "temperature":
		return "temperature"
	case "humidity":
		return "humidity"
	case "battery":
		return "battery"
	default:
		return ""
	}
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

func formatValue(value float64) string {
	body, _ := json.Marshal(map[string]any{"value": value})
	return string(body)
}
