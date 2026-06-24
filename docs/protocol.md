# UbiBot IoT Device Communication Protocol Specification

Version: v1.0
Date: 2026-06-24
Compatible firmware: ubibot-firmware-open v1.x+

---

## Table of Contents

1. [Overview](#1-overview)
2. [Sensor Field Definitions](#2-sensor-field-definitions)
3. [Common Data Formats](#3-common-data-formats)
4. [MQTT Protocol](#4-mqtt-protocol)
5. [HTTP Protocol](#5-http-protocol)
6. [CoAP Protocol](#6-coap-protocol)
7. [Device Configuration Push](#7-device-configuration-push)
8. [Authentication](#8-authentication)
9. [Error Codes](#9-error-codes)
10. [Firmware Integration Guide](#10-firmware-integration-guide)

---

## 1. Overview

The UbiBot platform supports three transport protocols. Device firmware may select the most appropriate protocol based on hardware capabilities and network conditions.

| Protocol | Port | Characteristics                         | Use Cases                               |
|----------|------|-----------------------------------------|-----------------------------------------|
| MQTT     | 1883 | Persistent connection, lightweight, bidirectional | Primary protocol; supports real-time config push |
| HTTP     | 8080 | Stateless, easy to debug                | Simple deployments; NAT-traversal-limited environments |
| CoAP     | 5683 | UDP-based, extremely low power          | Low-power MCUs, NB-IoT networks         |

All three protocols share the same JSON payload format. When a device uploads telemetry, the platform returns the current device configuration in the same response — **no separate config round-trip is required** (HTTP / CoAP).

---

## 2. Sensor Field Definitions

The platform supports the sensor types listed below. Firmware must use the exact key names shown (case-sensitive).

| Field Name         | Physical Quantity    | Unit  | Type  | Notes                                |
|--------------------|----------------------|-------|-------|--------------------------------------|
| `temperature`      | Temperature          | °C    | float | Ambient temperature                  |
| `humidity`         | Relative humidity    | % RH  | float | Ambient humidity                     |
| `light`            | Illuminance          | lux   | float | Visible light intensity              |
| `vibration`        | Vibration acceleration | m/s² | float | Combined tri-axis or single-axis peak |
| `soil_nitrogen`    | Soil nitrogen        | mg/kg | float | Available nitrogen in soil           |
| `soil_phosphorus`  | Soil phosphorus      | mg/kg | float | Available phosphorus in soil         |
| `soil_potassium`   | Soil potassium       | mg/kg | float | Available potassium in soil          |
| `battery`          | Battery level        | %     | float | Range 0–100                          |
| `signal`           | Wireless signal      | dBm   | int   | RSSI; negative value, higher is better |
| `pressure`         | Atmospheric pressure | hPa   | float | Barometric pressure                  |
| `co2`              | CO2 concentration    | ppm   | float | Carbon dioxide concentration         |

> **Custom fields**: Firmware may report fields not listed above. The platform stores them as-is but they do not participate in the rule engine.

---

## 3. Common Data Formats

### 3.1 Telemetry Uplink Payload (TelemetryPayload)

Used when a device sends sensor data to the platform. Shared across all three protocols.

```json
{
  "device_id": "DEV001",
  "token":     "a3f8c2e1d4b567890abcdef1234567890abcdef1234567890abcdef1234567",
  "timestamp": 1735000000,
  "sensors": {
    "temperature":      25.6,
    "humidity":         60.2,
    "light":            1200,
    "vibration":        0.02,
    "soil_nitrogen":    45,
    "soil_phosphorus":  30,
    "soil_potassium":   80,
    "battery":          85.0,
    "signal":          -67
  }
}
```

| Field       | Type   | Required | Notes                                                       |
|-------------|--------|----------|-------------------------------------------------------------|
| `device_id` | string | Yes*     | Unique device ID. May be omitted on MQTT (extracted from the topic). |
| `token`     | string | Yes*     | Device authentication token. May be omitted on MQTT when broker-level credentials are used. |
| `timestamp` | int64  | No       | Unix epoch in seconds. The platform substitutes the server time when absent or zero. |
| `sensors`   | object | Yes      | Must contain at least one sensor field.                     |

### 3.2 Configuration Downlink Payload (ConfigPayload)

Sent by the platform to a device to update its sampling parameters.

```json
{
  "collect_interval": 30,
  "upload_interval":  60,
  "sensors_enabled": ["temperature", "humidity", "light"],
  "server_time":      1735000000
}
```

| Field               | Type     | Notes                                                              |
|---------------------|----------|--------------------------------------------------------------------|
| `collect_interval`  | int      | How often (seconds) the firmware samples sensors.                  |
| `upload_interval`   | int      | How often (seconds) the firmware transmits buffered readings.      |
| `sensors_enabled`   | []string | Sensors the device should collect. Empty array means collect all.  |
| `server_time`       | int64    | Current Unix epoch stamped by the platform for device clock sync.  |

### 3.3 Heartbeat Payload (HeartbeatPayload, optional)

A device may send a heartbeat to signal liveness without uploading full telemetry.

```json
{
  "device_id": "DEV001",
  "token":     "...",
  "timestamp": 1735000000,
  "battery":   85.0,
  "signal":   -67
}
```

---

## 4. MQTT Protocol

### 4.1 Broker Address

```
mqtt://<platform-host>:1883
```

The current version allows all connections (development mode). Production deployments should enable username/password authentication.

### 4.2 Topic Design

| Direction | Topic                              | QoS | Notes                                        |
|-----------|------------------------------------|-----|----------------------------------------------|
| Uplink    | `ubibot/{device_id}/telemetry`     | 0   | Device publishes sensor telemetry.           |
| Uplink    | `ubibot/{device_id}/heartbeat`     | 0   | Device publishes a heartbeat.                |
| Downlink  | `ubibot/{device_id}/cmd/config`    | 1   | Platform pushes configuration (retained=true). |

`{device_id}` is the unique ID assigned at registration (e.g. `DEV001`).

### 4.3 Telemetry Uplink Example

**Topic**: `ubibot/DEV001/telemetry`

**Payload** (`device_id` and `token` may be omitted):

```json
{
  "sensors": {
    "temperature": 25.6,
    "humidity": 60.2,
    "soil_nitrogen": 45
  }
}
```

After receiving the message the platform publishes the current config as a retained message to `ubibot/DEV001/cmd/config`.

### 4.4 Configuration Receive Example

**Topic**: `ubibot/DEV001/cmd/config`

```json
{
  "collect_interval": 30,
  "upload_interval": 60,
  "sensors_enabled": ["temperature", "humidity", "soil_nitrogen"],
  "server_time": 1735000000
}
```

Firmware should read the retained message immediately after subscribing — no need to wait for a new push.

### 4.5 Firmware Implementation Notes

```
1. Connect to the broker (clientID = device_id).
2. Subscribe to ubibot/{device_id}/cmd/config (receives the retained config instantly).
3. Sample sensors every collect_interval seconds.
4. Publish telemetry every upload_interval seconds (batch or single reading).
5. Reconnect on disconnect using exponential back-off.
```

---

## 5. HTTP Protocol

### 5.1 Base URL

```
http://<platform-host>:8080
```

### 5.2 Device Telemetry Upload

**`POST /device/v1/telemetry`**

Authentication: supply the token in the `X-Device-Token` request header or in the JSON body's `token` field.

**Request example**:

```http
POST /device/v1/telemetry HTTP/1.1
Host: 192.168.1.100:8080
Content-Type: application/json
X-Device-Token: a3f8c2e1d4b567890abcdef123456789...

{
  "device_id": "DEV001",
  "timestamp": 1735000000,
  "sensors": {
    "temperature": 25.6,
    "humidity": 60.2,
    "light": 1200,
    "vibration": 0.02,
    "soil_nitrogen": 45,
    "soil_phosphorus": 30,
    "soil_potassium": 80,
    "battery": 85.0,
    "signal": -67
  }
}
```

**Success response (200 OK)**:

```json
{
  "collect_interval": 30,
  "upload_interval": 60,
  "sensors_enabled": [],
  "server_time": 1735000005
}
```

The response body is the current device configuration. Firmware does not need a separate config request.

**Error responses**:

| HTTP Status | Reason                                 |
|-------------|----------------------------------------|
| 400         | Malformed JSON or empty sensors map.   |
| 401         | Device not found or token mismatch.    |

### 5.3 Device Config Poll

**`GET /device/v1/config/{device_id}`**

No authentication required (config contains no sensitive data). Call on device boot or after reconnect.

**Request example**:

```http
GET /device/v1/config/DEV001 HTTP/1.1
Host: 192.168.1.100:8080
```

**Response (200 OK)**:

```json
{
  "collect_interval": 30,
  "upload_interval": 60,
  "sensors_enabled": ["temperature", "humidity"],
  "server_time": 1735000005
}
```

### 5.4 Platform Sets Device Config (Operator API)

**`PUT /api/devices/{device_id}/config`**

Called by platform operators or an app. Persists the new parameters and immediately pushes them to the device via a retained MQTT message.

**Request body**:

```json
{
  "collect_interval": 60,
  "upload_interval": 120,
  "sensors_enabled": ["temperature", "humidity", "soil_nitrogen", "soil_phosphorus", "soil_potassium"]
}
```

| Field               | Type     | Notes                                               |
|---------------------|----------|-----------------------------------------------------|
| `collect_interval`  | int      | Sampling interval in seconds. Minimum 1, default 30. |
| `upload_interval`   | int      | Upload interval in seconds. Minimum 1, default 60.   |
| `sensors_enabled`   | []string | Empty array enables all sensors.                    |

**Response (200 OK)**: ConfigPayload format (see section 3.2).

---

## 6. CoAP Protocol

### 6.1 Server Address

```
coap://<platform-host>:5683
```

UDP-based. Suitable for NB-IoT, LoRa and other constrained networks.

### 6.2 Resources

| Method | Path          | Description              |
|--------|---------------|--------------------------|
| PUT    | `/telemetry`  | Upload telemetry data.   |
| GET    | `/config`     | Fetch device config.     |

Content-Format: `application/json` (CoAP Content-Format code `50`).

### 6.3 Upload Telemetry (PUT /telemetry)

The payload is identical to the HTTP telemetry endpoint. `device_id` and `token` must be present in the JSON body.

```
PUT coap://192.168.1.100:5683/telemetry
Content-Format: application/json

{
  "device_id": "DEV001",
  "token": "a3f8c2e1...",
  "sensors": {
    "temperature": 25.6,
    "humidity": 60.2
  }
}
```

**Response code**: `2.04 Changed`
**Response payload**: ConfigPayload (JSON)

### 6.4 Fetch Config (GET /config)

Pass authentication via URI Query parameters:

```
GET coap://192.168.1.100:5683/config?device_id=DEV001&token=a3f8c2e1...
```

**Response code**: `2.05 Content`
**Response payload**: ConfigPayload (JSON)

### 6.5 CoAP Response Codes

| CoAP Code | Meaning                    |
|-----------|----------------------------|
| 2.04      | Telemetry accepted.        |
| 2.05      | Config returned.           |
| 4.00      | Malformed request.         |
| 4.01      | Authentication failed.     |
| 4.05      | Method not allowed.        |

---

## 7. Device Configuration Push

### 7.1 Configuration Parameters

| Parameter           | Range    | Default | Notes                                                           |
|---------------------|----------|---------|-----------------------------------------------------------------|
| `collect_interval`  | 1–86400  | 30      | Sensor sampling period in seconds. Lower values increase power consumption. |
| `upload_interval`   | 1–86400  | 60      | Data upload period in seconds. Should be >= `collect_interval`. |
| `sensors_enabled`   | any list | []      | Empty = collect all sensors; non-empty = collect only listed sensors. |

> **Recommendation**: Set `upload_interval` to a whole multiple of `collect_interval` so firmware can buffer multiple readings locally before transmitting.

### 7.2 Configuration Propagation Flow

```
                ┌──────────────────┐
  Operator       │  Platform REST   │
  PUT /config ──►│  PUT /api/       │
                 │  devices/DEV001/ │
                 │  config          │
                 └────────┬─────────┘
                          │ write to SQLite
                          │ + MQTT Publish (Retained)
                          ▼
                 ubibot/DEV001/cmd/config
                          │
                 ┌────────▼─────────┐
                 │  Device firmware │
                 │  subscribes and  │
                 │  updates params  │
                 └──────────────────┘
```

**How each protocol receives configuration**:

- **MQTT**: The platform publishes a retained message immediately. The device receives it upon connect without waiting for the next push.
- **HTTP**: The device receives the latest config in the response body of every telemetry upload.
- **CoAP**: The device receives config in the response to `PUT /telemetry`; it may also poll `GET /config` explicitly.

---

## 8. Authentication

### 8.1 Device Registration and Token Issuance

Before a device can connect, a platform administrator must register it:

```http
POST /api/devices HTTP/1.1
Content-Type: application/json

{
  "device_id": "DEV001",
  "name": "Greenhouse Sensor #1"
}
```

The platform returns a unique `token` that must be flashed into the device firmware:

```json
{
  "id": 1,
  "device_id": "DEV001",
  "name": "Greenhouse Sensor #1",
  "token": "a3f8c2e1d4b567890abcdef1234567890abcdef1234567890abcdef1234567",
  "online": false,
  "created_at": "2026-06-24T10:00:00Z"
}
```

### 8.2 Per-Protocol Authentication

| Protocol | Method                                                                          |
|----------|---------------------------------------------------------------------------------|
| MQTT     | Anonymous connections allowed in development mode. Production: set username=device_id, password=token on the broker. |
| HTTP     | `X-Device-Token` request header **or** `token` field in the JSON body.          |
| CoAP     | `token` field in the JSON body (telemetry); `?token=...` URI query (config poll). |

---

## 9. Error Codes

### HTTP Error Response Format

```json
{
  "error": "invalid token"
}
```

### Common Errors

| Scenario              | HTTP | CoAP | Description                                    |
|-----------------------|------|------|------------------------------------------------|
| JSON parse failure    | 400  | 4.00 | Check Content-Type header and JSON syntax.     |
| Empty sensors map     | 400  | 4.00 | At least one sensor field is required.         |
| Device not found      | 401  | 4.01 | Check whether device_id has been registered.   |
| Token mismatch        | 401  | 4.01 | Verify the token matches the registered value. |
| Internal server error | 500  | 5.00 | Contact the platform administrator.            |

---

## 10. Firmware Integration Guide

### 10.1 Recommended Integration Flow (MQTT)

```c
// Pseudo-code; assumes ESP-IDF / Arduino style API

void setup() {
    // 1. Read credentials from flash storage
    const char* device_id = "DEV001";
    const char* token     = "a3f8c2e1...";
    const char* server    = "192.168.1.100";

    // 2. Connect to the MQTT broker
    mqtt_connect(server, 1883, device_id, /*user*/NULL, /*pass*/NULL);

    // 3. Subscribe to config topic (receives Retained message immediately)
    char config_topic[64];
    snprintf(config_topic, sizeof(config_topic),
             "ubibot/%s/cmd/config", device_id);
    mqtt_subscribe(config_topic, on_config_received);
}

void on_config_received(const char* payload) {
    config.collect_interval = json_get_int(payload, "collect_interval");
    config.upload_interval  = json_get_int(payload, "upload_interval");
}

void loop() {
    if (time_to_collect()) {
        float temp = read_temperature();
        float humi = read_humidity();

        // 4. Publish telemetry
        char topic[64];
        snprintf(topic, sizeof(topic), "ubibot/%s/telemetry", device_id);
        char payload[256];
        snprintf(payload, sizeof(payload),
            "{\"sensors\":{\"temperature\":%.1f,\"humidity\":%.1f}}",
            temp, humi);
        mqtt_publish(topic, payload, QOS_0);
    }
    sleep(1);
}
```

### 10.2 Recommended Integration Flow (HTTP)

```c
// Polling mode; suitable for modules without persistent connection support

void loop() {
    if (time_to_upload()) {
        char body[512];
        snprintf(body, sizeof(body),
            "{\"device_id\":\"DEV001\","
             "\"token\":\"a3f8c2e1...\","
             "\"sensors\":{\"temperature\":%.1f,\"humidity\":%.1f}}",
            temp, humi);

        char response[512];
        http_post("http://192.168.1.100:8080/device/v1/telemetry",
                  body, response);

        // Update sampling config from the response
        config.collect_interval = json_get_int(response, "collect_interval");
        config.upload_interval  = json_get_int(response, "upload_interval");
    }
}
```

### 10.3 Soil NPK Sensor Notes

Soil nitrogen/phosphorus/potassium sensors typically communicate with the MCU via RS-485 / Modbus. A sampling interval of at least 30 seconds is recommended to allow sensor warm-up. Report readings using the standard field names:

```json
{
  "sensors": {
    "soil_nitrogen":   45.2,
    "soil_phosphorus": 28.7,
    "soil_potassium":  76.1,
    "temperature":     22.5,
    "humidity":        58.3
  }
}
```

### 10.4 Vibration Sensor Notes

Vibration values are in m/s². Report the combined tri-axis RMS acceleration or a single-axis peak value. The sampling rate is controlled by `collect_interval`. For high-frequency monitoring scenarios (e.g. equipment fault detection), compute statistical summaries (peak, RMS) locally on the device and report only the summary to reduce bandwidth usage.

---

*This document is maintained alongside `internal/protocol/payload.go` in the ubibot-platform-open source tree.*
