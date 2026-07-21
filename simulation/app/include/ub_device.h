/* Device state machine: cold/warm-boot activation, periodic sampling and
 * reporting, command dispatch (set_cfg/reboot/calibrate/set_probe/ota), and
 * OTA download/apply. This is the part of the simulator that plays the
 * role of firmware's main application task -- it only calls into
 * ub_protocol.h (codec), ub_transport.h (HTTP), ub_platform.h (clock/sleep)
 * and ub_storage.h (persistence), so it is itself portable to FreeRTOS
 * unchanged; only the four HAL headers it depends on need a firmware-side
 * implementation. */
#ifndef UB_DEVICE_H
#define UB_DEVICE_H

#include <stdint.h>

#include "ub_protocol.h"
#include "ub_transport.h"

#ifdef __cplusplus
extern "C" {
#endif

#define UB_DEV_HOST_MAX 128
#define UB_DEV_DIR_MAX 256
#define UB_DEV_SECRET_MAX 128
#define UB_MAX_PROBES 8
#define UB_MAX_PENDING_ACKS 8

typedef struct {
    char pid[UB_PROBE_PID_MAX];
    char key[UB_PROBE_KEY_MAX];
    char iface[UB_PROBE_IFACE_MAX];
    char proto[UB_PROBE_PROTO_MAX];
    double scale;
    double offset;
    int in_use;
} ub_probe_t;

typedef enum {
    UB_OTA_IDLE = 0,
    UB_OTA_DOWNLOADING,
    UB_OTA_VERIFYING,
    UB_OTA_FLASHING,
    UB_OTA_REBOOTING,
    UB_OTA_SUCCESS,
    UB_OTA_FAILED,
    UB_OTA_ROLLED_BACK
} ub_ota_state_t;

typedef struct {
    ub_ota_state_t state;
    char cmd_id[UB_CMD_ID_MAX];
    char version[UB_OTA_VERSION_MAX];
    char url[192];
    long total_size;
    long downloaded;
    char sha256_hex[65];
    int cancel_requested;
    /* Set after a successful flash+reboot to run the post-reboot self-check
     * handshake (ack success / nak rolled_back) exactly once. */
    int pending_self_check;
} ub_ota_ctx_t;

/* One queued outcome from processing a command, reported on the *next*
 * report/poll cycle (never inline with the response that delivered the
 * command) — mirrors how a real device can't always finish acting on a
 * command within the same HTTP round trip. */
typedef struct {
    char cmd_id[UB_CMD_ID_MAX];
    int is_nak; /* 0 = ack, 1 = nak */
    int code;
    char msg[64];
} ub_pending_result_t;

typedef struct {
    /* identity */
    char pid[64];
    char sn[64];
    char secret[UB_DEV_SECRET_MAX];
    char server_host[UB_DEV_HOST_MAX];
    int server_port;
    char data_dir[UB_DEV_DIR_MAX];
} ub_device_config_t;

typedef struct {
    ub_device_config_t cfg;
    const ub_transport_t *transport;

    /* session */
    char token[UB_TOKEN_MAX];
    int64_t token_expires_at;
    int has_clock_ref; /* true once local wall clock is known good enough to warm-boot activate */

    /* server-assigned config */
    int ci; /* sample interval, seconds */
    int ui; /* upload interval, seconds */
    int fe_count;
    char fe[UB_MAX_FE][UB_FE_NAME_MAX];

    /* custom probes (protocol §7.2) */
    ub_probe_t probes[UB_MAX_PROBES];
    int probes_dirty; /* set on any set_probe apply; triggers a "prb" reconciliation report */

    /* commands not yet acked/naked */
    ub_pending_result_t pending[UB_MAX_PENDING_ACKS];
    int pending_count;

    ub_ota_ctx_t ota;

    /* timers (all monotonic ms) */
    uint64_t last_sample_ms;
    uint64_t last_report_ms;
    uint64_t last_poll_ms;

    /* buffered samples awaiting the next report */
    struct {
        int64_t ts;
        double temperature;
        double humidity;
        int has_temperature;
        int has_humidity;
    } sample_buf[16];
    int sample_count;

    int reboot_requested; /* set by the "reboot" command; acted on at the end of the current tick */
    int running;
} ub_device_t;

/* Zeroes *dev, copies cfg in, and attempts to restore session/config state
 * previously saved by ub_device_persist (a fresh device with nothing saved
 * simply starts unactivated). */
void ub_device_init(ub_device_t *dev, const ub_device_config_t *cfg, const ub_transport_t *transport);

/* Saves the fields that need to survive a restart (token/exp, ci/ui/fe,
 * probes, has_clock_ref) via ub_storage_save. Call after anything that
 * changes them. */
void ub_device_persist(ub_device_t *dev);

/* Does one unit of work appropriate for right now (activation if needed,
 * sampling if due, reporting if due, continuing an in-flight OTA) and
 * returns. Intended to be called in a loop from main(), e.g. once per
 * second; safe to call more or less often since it's driven by
 * ub_platform_monotonic_ms() internally, not by call frequency. */
void ub_device_tick(ub_device_t *dev);

#ifdef __cplusplus
}
#endif

#endif /* UB_DEVICE_H */
