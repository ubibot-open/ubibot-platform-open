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

// Package config loads the platform configuration from a YAML file.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		HTTPPort int `yaml:"http_port"`
		MQTTPort int `yaml:"mqtt_port"`
		CoAPPort int `yaml:"coap_port"`
	} `yaml:"server"`

	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`

	Telemetry struct {
		BatchSize     int           `yaml:"batch_size"`
		FlushInterval time.Duration `yaml:"flush_interval"`
		RetentionDays int           `yaml:"retention_days"`
	} `yaml:"telemetry"`

	HomeAssistant struct {
		Enabled         bool   `yaml:"enabled"`
		Broker          string `yaml:"broker"`
		Username        string `yaml:"username"`
		Password        string `yaml:"password"`
		DiscoveryPrefix string `yaml:"discovery_prefix"`
		ClientID        string `yaml:"client_id"`
	} `yaml:"homeassistant"`

	Log struct {
		Level string `yaml:"level"`
	} `yaml:"log"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	c.applyDefaults()
	return &c, nil
}

func (c *Config) applyDefaults() {
	if c.Server.HTTPPort == 0 {
		c.Server.HTTPPort = 8080
	}
	if c.Server.MQTTPort == 0 {
		c.Server.MQTTPort = 1883
	}
	if c.Server.CoAPPort == 0 {
		c.Server.CoAPPort = 5683
	}
	if c.Database.Path == "" {
		c.Database.Path = "./data/ubibot.db"
	}
	if c.Telemetry.BatchSize == 0 {
		c.Telemetry.BatchSize = 100
	}
	if c.Telemetry.FlushInterval == 0 {
		c.Telemetry.FlushInterval = 2 * time.Second
	}
	if c.Telemetry.RetentionDays == 0 {
		c.Telemetry.RetentionDays = 30
	}
	if c.HomeAssistant.DiscoveryPrefix == "" {
		c.HomeAssistant.DiscoveryPrefix = "homeassistant"
	}
	if c.HomeAssistant.ClientID == "" {
		c.HomeAssistant.ClientID = "ubibot-platform"
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
}
