/* Device state machine: periodic sampling and batched reporting. This is the
 * part of the simulator that plays the role of firmware's main application
 * task -- it only calls into ub_protocol.h (codec), ub_transport.h (HTTP),
 * and ub_platform.h (clock/sleep), so it is itself portable to FreeRTOS
 * unchanged; only those three HAL headers need a firmware-side
 * implementation.
 *
 * There is no activation step and nothing worth persisting across a
 * restart: identity is just pid+sn (known at compile/CLI time, not learned
 * from the server), and there is no session token or server-pushed config
 * to survive a reboot. So, unlike the old protocol version, this module has
 * no ub_storage.h dependency at all -- a fresh process each time is
 * indistinguishable from a "warm" one. */
#ifndef UB_DEVICE_H
#define UB_DEVICE_H

#include <stdint.h>

#include "ub_protocol.h"
#include "ub_transport.h"

#ifdef __cplusplus
extern "C" {
#endif

#define UB_DEV_HOST_MAX 128

/* How many samples can be buffered locally before a report is forced (see
 * ub_device_tick). Also the effective cap on "offline period" batching. */
#define UB_SAMPLE_BUF_CAP 16

typedef struct {
    char pid[64];
    char sn[64];
    char server_host[UB_DEV_HOST_MAX];
    int server_port;
} ub_device_config_t;

typedef struct {
    int64_t ts;
    double value[UB_MAX_FIELDS]; /* value[0] is field1, value[1] is field2, ... */
    int has_value[UB_MAX_FIELDS];
} ub_sample_t;

typedef struct {
    ub_device_config_t cfg;
    const ub_transport_t *transport;

    /* how often to sample sensors / flush a report, in seconds -- fixed
     * local constants set in ub_device_init (see ub_device.c); there is no
     * server-pushed config channel to change them at runtime any more. */
    int sample_interval_sec;
    int report_interval_sec;

    uint64_t last_sample_ms; /* monotonic ms of the last sample, 0 = never */
    uint64_t last_report_ms; /* monotonic ms of the last report attempt, 0 = never */

    /* buffered samples awaiting the next report */
    ub_sample_t sample_buf[UB_SAMPLE_BUF_CAP];
    int sample_count;

    int running;
} ub_device_t;

/* Zeroes *dev, copies cfg in, and applies the default sample/report
 * intervals. Nothing is restored from disk -- see the file header comment
 * for why there is no persisted state to restore. */
void ub_device_init(ub_device_t *dev, const ub_device_config_t *cfg, const ub_transport_t *transport);

/* Does one unit of work appropriate for right now (sampling if due,
 * reporting if due) and returns. Intended to be called in a loop from
 * main(), e.g. once per second; safe to call more or less often since it's
 * driven by ub_platform_monotonic_ms() internally, not by call frequency. */
void ub_device_tick(ub_device_t *dev);

#ifdef __cplusplus
}
#endif

#endif /* UB_DEVICE_H */
