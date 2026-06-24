# UbiBot IoT Device Communication Protocol Specification

Version: v2.0
Date: 2026-06-24
Compatible firmware: ubibot-firmware-open v2.x+

---

## Table of Contents

1. [Overview](#1-overview)
2. [Field Slot Design](#2-field-slot-design)
3. [Common Data Formats](#3-common-data-formats)
4. [MQTT Protocol](#4-mqtt-protocol)
5. [HTTP Protocol](#5-http-protocol)
6. [CoAP Protocol](#6-coap-protocol)
7. [Device Configuration Push](#7-device-configuration-push)
8. [Field Definition Management](#8-field-definition-management)
9. [Authentication](#9-authentication)
10. [Error Codes](#10-error-codes)
11. [Firmware Integration Guide](#11-firmware-integration-guide)

---

## 1. Overview

The UbiBot platform supports three transport protocols. Device firmware may select the most appropriate protocol based on hardware capabilities and network conditions.

| Protocol | Port | Characteristics                          | Use Cases                                |
|----------|------|------------------------------------------|------------------------------------------|
| MQTT     | 1883 | Persistent connection, lightweight, bidirectional | Primary protocol; supports real-time config push |
| HTTP     | 8080 | Stateless, easy to debug                 | Simple deployments; NAT-traversal-limited environments |
| CoAP     | 5683 | UDP-based, extremely low power           | Low-power MCUs, NB-IoT networks          |

All three protocols share the same JSON payload format. When a device uploads data, the platform returns the current device configuration in the same response — **no separate config round-trip is required** (HTTP / CoAP).

---

## 2. Field Slot Design

Rather than using named sensor fields, the protocol uses **generic field slots** (`field1` through `field20`). The physical meaning of each slot is configured by the operator in the platform backend and has no bearing on the wire format.

| Key      | Type   | Notes                                                      |
|----------|--------|------------------------------------------------------------|
| `field1` | string | General-purpose slot. Assign any sensor value.             |
| `field2` | string |                                                            |
| …        | …      |                                                            |
| `field20`| string | Up to 20 slots available per device.                       |

**All field values are transmitted and stored as strings.** Numeric values should be formatted as their decimal representation (e.g. `"25.6"`). Boolean values may use `"0"`/`"1"` or `"true"`/`"false"`. The platform stores the raw string without type coercion.

The mapping from field key → physical meaning (display name, unit, description) is managed through the [Field Definition API](#8-field-definition-management) and is never embedded in the wire format.

---

## 3. Common Data Formats

### 3.1 Data Point

A **DataPoint** holds one timed measurement. A single upload may carry multiple DataPoints, allowing devices to batch readings collected between upload cycles.

```json
{
  "timestamp": 1735000000,
  "field1": "25.6",
  "field2": "60.2",
  "field3": "1200",
  "field4": "0.02"
}
```

| Field       | Type   | Required | Notes                                                      |
|-------------|--------|----------|------------------------------------------------------------|
| `timestamp` | int64  | No       | Unix epoch in seconds at collection time. The platform substitutes the server receive time when absent or zero. |
| `field1`…`field20` | string | At least one | Raw value from the sensor. Omit unused slots. |

### 3.2 Telemetry Uplink Payload (TelemetryPayload)

The top-level message a device sends to the platform. Shared across all three protocols.

```json
{
  "device_id": "DEV001",
  "token":     "a3f8c2e1d4b567890abcdef1234567890abcdef1234567890abcdef1234567",
  "data": [
    {
      "timestamp": 1735000000,
      "field1": "25.6",
      "field2": "60.2",
      "field3": "1200"
    },
    {
      "timestamp": 1735000030,
      "field1": "25.8",
      "field2": "59.9",
      "field3": "1180",
      "field4": "0.03"
    }
  ]
}
```

| Field       | Type         | Required | Notes                                                       |
|-------------|--------------|----------|-------------------------------------------------------------|
| `device_id` | string       | Yes*     | Unique device ID. May be omitted on MQTT (extracted from the topic). |
| `token`     | string       | Yes*     | Authentication token. May be omitted on MQTT when broker credentials are used. |
| `data`      | []DataPoint  | Yes      | One or more data points. Must be non-empty.                 |

### 3.3 Configuration Downlink Payload (ConfigPayload)

Sent by the platform to a device to control sampling and upload behaviour.

```json
{
  "collect_interval": 30,
  "upload_interval":  60,
  "fields_enabled": ["field1", "field2", "field3"],
  "server_time":      1735000000
}
```

| Field               | Type     | Notes                                                              |
|---------------------|----------|--------------------------------------------------------------------|
| `collect_interval`  | int      | How often (seconds) the firmware samples sensors.                  |
| `upload_interval`   | int      | How often (seconds) the firmware transmits buffered data points.   |
| `fields_enabled`    | []string | Field keys the device should collect. Empty array means collect all. |
| `server_time`       | int64    | Current Unix epoch stamped by the platform for device clock sync.  |

### 3.4 Heartbeat Payload (optional)

A lightweight message a device sends to signal liveness without a full data upload. Uses the same top-level structure with a `data` array containing a single DataPoint with only the most relevant fields (e.g. battery, signal).

```json
{
  "device_id": "DEV001",
  "token":     "...",
  "data": [
    { "timestamp": 1735000000, "field19": "85.0", "field20": "-67" }
  ]
}
```

By convention, operators may configure `field19` = Battery (%) and `field20` = Signal (dBm) for heartbeat-only fields, but this is entirely user-defined.

---

## 4. MQTT Protocol

### 4.1 Broker Address

```
mqtt://<platform-host>:1883
```

The current version allows all connections (development mode). Production deployments should enable username/password authentication.

### 4.2 Topic Design

| Direction | Topic                              | QoS | Notes                                         |
|-----------|------------------------------------|-----|-----------------------------------------------|
| Uplink    | `ubibot/{device_id}/telemetry`     | 0   | Device publishes one TelemetryPayload.        |
| Uplink    | `ubibot/{device_id}/heartbeat`     | 0   | Device publishes a lightweight heartbeat.     |
| Downlink  | `ubibot/{device_id}/cmd/config`    | 1   | Platform pushes ConfigPayload (retained=true). |

`{device_id}` is the unique ID assigned at registration (e.g. `DEV001`).

### 4.3 Telemetry Uplink Example

**Topic**: `ubibot/DEV001/telemetry`

**Payload** (two batched readings; `device_id` and `token` may be omitted on MQTT):

```json
{
  "data": [
    { "timestamp": 1735000000, "field1": "25.6", "field2": "60.2", "field3": "45" },
    { "timestamp": 1735000030, "field1": "25.8", "field2": "59.9", "field3": "46" }
  ]
}
```

After processing, the platform publishes the current config as a retained message on `ubibot/DEV001/cmd/config`.

### 4.4 Configuration Receive Example

**Topic**: `ubibot/DEV001/cmd/config`

```json
{
  "collect_interval": 30,
  "upload_interval": 60,
  "fields_enabled": ["field1", "field2", "field3"],
  "server_time": 1735000000
}
```

Firmware should read the retained message immediately after subscribing — no need to wait for a new push.

### 4.5 Firmware Implementation Notes

```
1. Connect to the broker (clientID = device_id).
2. Subscribe to ubibot/{device_id}/cmd/config (receives retained config instantly).
3. Sample the configured field slots every collect_interval seconds.
4. Buffer DataPoints locally and publish them in a single TelemetryPayload
   every upload_interval seconds.
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

**Request example** (three data points batched in one upload):

```http
POST /device/v1/telemetry HTTP/1.1
Host: 192.168.1.100:8080
Content-Type: application/json
X-Device-Token: a3f8c2e1d4b567890abcdef123456789...

{
  "device_id": "DEV001",
  "data": [
    { "timestamp": 1735000000, "field1": "25.6", "field2": "60.2", "field3": "45",  "field4": "30",  "field5": "80"  },
    { "timestamp": 1735000030, "field1": "25.8", "field2": "59.9", "field3": "45",  "field4": "31",  "field5": "79"  },
    { "timestamp": 1735000060, "field1": "26.1", "field2": "58.5", "field3": "46",  "field4": "30",  "field5": "81"  }
  ]
}
```

A typical field assignment in this example (configured by the operator via the Field Definition API):

| Field   | Physical Meaning   | Unit  |
|---------|--------------------|-------|
| field1  | Temperature        | °C    |
| field2  | Humidity           | % RH  |
| field3  | Soil Nitrogen      | mg/kg |
| field4  | Soil Phosphorus    | mg/kg |
| field5  | Soil Potassium     | mg/kg |

**Success response (200 OK)**:

```json
{
  "collect_interval": 30,
  "upload_interval": 60,
  "fields_enabled": ["field1", "field2", "field3", "field4", "field5"],
  "server_time": 1735000065
}
```

The response body is the current ConfigPayload. Firmware does not need a separate config request.

**Error responses**:

| HTTP Status | Reason                                   |
|-------------|------------------------------------------|
| 400         | Malformed JSON, empty data array, or all data points are empty. |
| 401         | Device not found or token mismatch.      |

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
  "fields_enabled": ["field1", "field2"],
  "server_time": 1735000065
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
  "fields_enabled": ["field1", "field2", "field3", "field4", "field5"]
}
```

| Field               | Type     | Notes                                                |
|---------------------|----------|------------------------------------------------------|
| `collect_interval`  | int      | Sampling interval in seconds. Minimum 1, default 30. |
| `upload_interval`   | int      | Upload interval in seconds. Minimum 1, default 60.   |
| `fields_enabled`    | []string | Empty array enables all field slots.                 |

**Response (200 OK)**: ConfigPayload format.

---

## 6. CoAP Protocol

### 6.1 Server Address

```
coap://<platform-host>:5683
```

UDP-based. Suitable for NB-IoT, LoRa and other constrained networks.

### 6.2 Resources

| Method | Path          | Description                |
|--------|---------------|----------------------------|
| PUT    | `/telemetry`  | Upload a TelemetryPayload. |
| GET    | `/config`     | Fetch the device config.   |

Content-Format: `application/json` (CoAP Content-Format code `50`).

### 6.3 Upload Telemetry (PUT /telemetry)

The payload is identical to the HTTP telemetry endpoint. `device_id` and `token` must be present in the JSON body.

```
PUT coap://192.168.1.100:5683/telemetry
Content-Format: application/json

{
  "device_id": "DEV001",
  "token": "a3f8c2e1...",
  "data": [
    { "timestamp": 1735000000, "field1": "25.6", "field2": "60.2" }
  ]
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

| CoAP Code | Meaning                  |
|-----------|--------------------------|
| 2.04      | Telemetry accepted.      |
| 2.05      | Config returned.         |
| 4.00      | Malformed request.       |
| 4.01      | Authentication failed.   |
| 4.05      | Method not allowed.      |

---

## 7. Device Configuration Push

### 7.1 Configuration Parameters

| Parameter           | Range    | Default | Notes                                                               |
|---------------------|----------|---------|---------------------------------------------------------------------|
| `collect_interval`  | 1–86400  | 30      | Sensor sampling period in seconds.                                  |
| `upload_interval`   | 1–86400  | 60      | Data upload period. Should be ≥ `collect_interval`.                 |
| `fields_enabled`    | any list | []      | Empty = collect all slots; non-empty = collect only listed slots.   |

> **Recommendation**: Set `upload_interval` to a whole multiple of `collect_interval`. Firmware buffers multiple DataPoints locally and sends them in one payload, minimising radio wake-ups.

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

## 8. Field Definition Management

Operators configure what each field slot means through the Field Definition API. Definitions are stored per-device; a definition with `device_id = "_default"` acts as a global fallback for all devices.

### 8.1 Field Definition Object

```json
{
  "id": 1,
  "device_id": "DEV001",
  "field_key": "field1",
  "display_name": "Temperature",
  "unit": "°C",
  "description": "Ambient temperature from DHT22 sensor",
  "updated_at": "2026-06-24T10:00:00Z"
}
```

### 8.2 List Field Definitions

**`GET /api/field-definitions`**

Query parameter `?device_id=DEV001` filters by device (returns both device-specific and global defaults).

**Response (200 OK)**:

```json
[
  { "id": 1, "device_id": "DEV001", "field_key": "field1", "display_name": "Temperature", "unit": "°C", "description": "" },
  { "id": 2, "device_id": "DEV001", "field_key": "field2", "display_name": "Humidity",    "unit": "%",  "description": "" },
  { "id": 3, "device_id": "DEV001", "field_key": "field3", "display_name": "Soil N",      "unit": "mg/kg", "description": "Soil available nitrogen" }
]
```

### 8.3 Create or Update a Field Definition

**`PUT /api/field-definitions/{device_id}/{field_key}`**

Use `_default` as `{device_id}` to set a global default visible to all devices that lack a device-specific definition.

**Request body**:

```json
{
  "display_name": "Soil Nitrogen",
  "unit": "mg/kg",
  "description": "Available nitrogen measured by RS-485 sensor"
}
```

**Response (200 OK)**: the saved FieldDefinition object.

**Example — configure field1–field7 for a greenhouse device**:

```
PUT /api/field-definitions/DEV001/field1   { "display_name": "Temperature",   "unit": "°C"    }
PUT /api/field-definitions/DEV001/field2   { "display_name": "Humidity",       "unit": "%"     }
PUT /api/field-definitions/DEV001/field3   { "display_name": "Light",          "unit": "lux"   }
PUT /api/field-definitions/DEV001/field4   { "display_name": "Vibration",      "unit": "m/s²"  }
PUT /api/field-definitions/DEV001/field5   { "display_name": "Soil Nitrogen",   "unit": "mg/kg" }
PUT /api/field-definitions/DEV001/field6   { "display_name": "Soil Phosphorus", "unit": "mg/kg" }
PUT /api/field-definitions/DEV001/field7   { "display_name": "Soil Potassium",  "unit": "mg/kg" }
```

### 8.4 Delete a Field Definition

**`DELETE /api/field-definitions/{device_id}/{field_key}`**

Returns `204 No Content` on success.

---

## 9. Authentication

### 9.1 Device Registration and Token Issuance

Before a device can connect, a platform administrator must register it:

```http
POST /api/devices HTTP/1.1
Content-Type: application/json

{
  "device_id": "DEV001",
  "name": "Greenhouse Sensor #1"
}
```

The platform returns a unique `token` that must be stored in the device firmware:

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

### 9.2 Per-Protocol Authentication

| Protocol | Method                                                                          |
|----------|---------------------------------------------------------------------------------|
| MQTT     | Anonymous connections allowed in development mode. Production: set username=device_id, password=token on the broker. |
| HTTP     | `X-Device-Token` request header **or** `token` field in the JSON body.          |
| CoAP     | `token` field in the JSON body (telemetry); `?token=...` URI query (config poll). |

---

## 10. Error Codes

### HTTP Error Response Format

```json
{
  "error": "invalid token"
}
```

### Common Errors

| Scenario                          | HTTP | CoAP | Description                                      |
|-----------------------------------|------|------|--------------------------------------------------|
| JSON parse failure                | 400  | 4.00 | Check Content-Type header and JSON syntax.       |
| Empty `data` array                | 400  | 4.00 | At least one DataPoint with one field required.  |
| All DataPoints are empty          | 400  | 4.00 | Each DataPoint must have at least one field.     |
| Device not found                  | 401  | 4.01 | Check whether device_id has been registered.     |
| Token mismatch                    | 401  | 4.01 | Verify the token matches the registered value.   |
| Internal server error             | 500  | 5.00 | Contact the platform administrator.              |

---

## 11. Firmware Integration Guide

### 11.1 Recommended Integration Flow (MQTT)

```c
// Pseudo-code; assumes ESP-IDF / Arduino style API

#define COLLECT_INTERVAL 30   // seconds; overridden by ConfigPayload
#define UPLOAD_INTERVAL  60

void setup() {
    const char* device_id = "DEV001";
    const char* token     = "a3f8c2e1...";
    const char* server    = "192.168.1.100";

    mqtt_connect(server, 1883, device_id, NULL, NULL);

    // Subscribe; retained message delivers latest config immediately.
    char topic[64];
    snprintf(topic, sizeof(topic), "ubibot/%s/cmd/config", device_id);
    mqtt_subscribe(topic, on_config);
}

void on_config(const char* payload) {
    cfg.collect_interval = json_get_int(payload, "collect_interval");
    cfg.upload_interval  = json_get_int(payload, "upload_interval");
}

// ---- Collect loop ----
DataPoint batch[8];
int batch_count = 0;

void collect_tick() {
    DataPoint* dp = &batch[batch_count++];
    dp->timestamp = unix_time();
    dp->field1    = float_to_str(read_temperature());   // e.g. "25.6"
    dp->field2    = float_to_str(read_humidity());
    dp->field3    = float_to_str(read_soil_nitrogen());
    dp->field4    = float_to_str(read_soil_phosphorus());
    dp->field5    = float_to_str(read_soil_potassium());
}

// ---- Upload loop ----
void upload_tick() {
    // Build {"data": [ {...}, {...} ]}
    char payload[2048];
    build_telemetry_json(payload, batch, batch_count);

    char topic[64];
    snprintf(topic, sizeof(topic), "ubibot/%s/telemetry", device_id);
    mqtt_publish(topic, payload, QOS_0);

    batch_count = 0; // reset buffer
}
```

### 11.2 Recommended Integration Flow (HTTP)

```c
// Polling mode; no persistent connection required.

void upload_tick() {
    // Buffer multiple DataPoints since the last upload.
    char body[2048];
    snprintf(body, sizeof(body),
        "{"
        "\"device_id\":\"DEV001\","
        "\"token\":\"a3f8c2e1...\","
        "\"data\":["
          "{\"timestamp\":%ld,\"field1\":\"%s\",\"field2\":\"%s\"},"
          "{\"timestamp\":%ld,\"field1\":\"%s\",\"field2\":\"%s\"}"
        "]}",
        ts1, f1_val1, f2_val1,
        ts2, f1_val2, f2_val2);

    char response[512];
    http_post("http://192.168.1.100:8080/device/v1/telemetry",
              body, response);

    // Parse ConfigPayload from the response to update intervals.
    cfg.collect_interval = json_get_int(response, "collect_interval");
    cfg.upload_interval  = json_get_int(response, "upload_interval");
}
```

### 11.3 Field Assignment Examples

The wire format is agnostic to physical meaning. Below are two common field assignments; configure them via the Field Definition API.

**Greenhouse multi-sensor node**:

| Slot    | Physical Meaning   | Unit  | Sensor Type             |
|---------|--------------------|-------|-------------------------|
| field1  | Temperature        | °C    | DHT22 / SHT31           |
| field2  | Humidity           | % RH  | DHT22 / SHT31           |
| field3  | Light              | lux   | BH1750                  |
| field4  | Soil Nitrogen      | mg/kg | RS-485 NPK sensor       |
| field5  | Soil Phosphorus    | mg/kg | RS-485 NPK sensor       |
| field6  | Soil Potassium     | mg/kg | RS-485 NPK sensor       |
| field7  | CO2                | ppm   | MH-Z19 / SCD40          |
| field19 | Battery            | %     | ADC measurement         |
| field20 | Signal (RSSI)      | dBm   | WiFi / LoRa driver      |

**Industrial vibration monitor**:

| Slot    | Physical Meaning       | Unit  | Notes                          |
|---------|------------------------|-------|--------------------------------|
| field1  | Vibration RMS X        | m/s²  | Tri-axis accelerometer         |
| field2  | Vibration RMS Y        | m/s²  |                                |
| field3  | Vibration RMS Z        | m/s²  |                                |
| field4  | Vibration Peak         | m/s²  | Combined tri-axis peak         |
| field5  | Temperature (bearing)  | °C    | NTC thermistor                 |
| field19 | Battery                | %     |                                |
| field20 | Signal                 | dBm   |                                |

### 11.4 RS-485 / Modbus Sensor Notes

Soil NPK sensors and similar Modbus devices require a warm-up period of at least 30 seconds. Set `collect_interval` ≥ 30 for these sensors. For devices with multiple Modbus sensors on the same bus, serialize requests and assign results to consecutive field slots.

### 11.5 String Value Formatting Rules

| Physical Type | Recommended Format           | Example       |
|---------------|------------------------------|---------------|
| Float         | Decimal, no trailing zeros   | `"25.6"`      |
| Integer       | Plain integer                | `"1200"`      |
| Negative      | Standard minus sign          | `"-67"`       |
| Boolean       | `"0"` / `"1"`                | `"1"`         |
| Enum/status   | Short ASCII string           | `"on"`, `"fault"` |

The rule engine parses string values as float64 when evaluating numeric thresholds. Non-numeric strings are silently skipped for threshold rules but are stored and queryable.

---

*This document is maintained alongside `internal/protocol/payload.go` in the ubibot-platform-open source tree.*
