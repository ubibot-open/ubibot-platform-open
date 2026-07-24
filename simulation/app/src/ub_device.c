/* Device state machine implementation. See ub_device.h for the module's
 * role in the overall architecture. Everything below only calls into
 * ub_protocol.h/ub_transport.h/ub_platform.h/ub_sensors.h, so it compiles
 * and runs unmodified once those four headers have a firmware
 * implementation. */
#include "ub_device.h"

#include <string.h>

#include "ub_platform.h"
#include "ub_sensors.h"

/* Fixed local defaults: how often to read sensors, and how often to flush
 * a report. The old protocol let the server push these via a "set_cfg"
 * command; that whole channel is gone now, so these are just constants.
 * (Could equally be made CLI flags in main.c -- kept as constants here to
 * keep the surface small; see simulation/README.md.) */
#define UB_DEFAULT_SAMPLE_INTERVAL_SEC 30
#define UB_DEFAULT_REPORT_INTERVAL_SEC 300

void ub_device_init(ub_device_t *dev, const ub_device_config_t *cfg, const ub_transport_t *transport) {
    memset(dev, 0, sizeof(*dev));
    dev->cfg = *cfg;
    dev->transport = transport;
    dev->sample_interval_sec = UB_DEFAULT_SAMPLE_INTERVAL_SEC;
    dev->report_interval_sec = UB_DEFAULT_REPORT_INTERVAL_SEC;
    dev->running = 1;

    ub_platform_log("device initialized: pid=%s sn=%s sample_interval=%ds report_interval=%ds",
                     dev->cfg.pid, dev->cfg.sn, dev->sample_interval_sec, dev->report_interval_sec);
}

/* ---- time sync (protocol §3) ---------------------------------------------
 * No auth, no signature: a convenience for a device with no RTC (or one
 * that just noticed its clock is out of the server's +-5 minute tolerance
 * window, see the 1002 handling in do_report below). */
static int do_time_sync(ub_device_t *dev) {
    ub_identity_t id = {dev->cfg.pid, dev->cfg.sn};
    char req[128];
    int n = ub_build_time_request(&id, req, sizeof(req));
    if (n < 0) return -1;

    char resp[256];
    int status;
    size_t resp_len;
    if (dev->transport->post(dev->transport->user_ctx, dev->cfg.server_host, dev->cfg.server_port,
                              "/api/v1/auth/time", req, (size_t)n, &status, resp, sizeof(resp),
                              &resp_len) != 0) {
        ub_platform_log("time sync: transport error");
        return -1;
    }

    ub_time_response_t tr;
    if (ub_parse_time_response(resp, &tr) != 0 || tr.c != UB_CODE_OK) {
        ub_platform_log("time sync rejected (http=%d c=%d)", status, tr.c);
        return -1;
    }
    ub_platform_set_time(tr.t);
    ub_platform_log("time sync ok: server time=%lld", (long long)tr.t);
    return 0;
}

/* ---- sampling (protocol §5) -----------------------------------------------
 * field1=temperature, field2=humidity, field3=light -- the three fields
 * with a conventional meaning; field4-20 are left to whoever adds more
 * sensors (not exercised by this reference device). */
static void sample(ub_device_t *dev) {
    if (dev->sample_count >= UB_SAMPLE_BUF_CAP) {
        ub_platform_log("sample buffer full, dropping this reading until the next report flushes it");
        return;
    }

    ub_sample_t *s = &dev->sample_buf[dev->sample_count++];
    memset(s, 0, sizeof(*s));
    s->ts = ub_platform_now();

    s->value[0] = ub_sensor_read_temperature();
    s->has_value[0] = 1;
    s->value[1] = ub_sensor_read_humidity();
    s->has_value[1] = 1;
    s->value[2] = ub_sensor_read_light();
    s->has_value[2] = 1;
}

/* ---- reporting (protocol §4) ----------------------------------------------*/
static void do_report(ub_device_t *dev) {
    static char buf[4096];
    ub_report_builder_t b;
    ub_report_begin(&b, buf, sizeof(buf), dev->cfg.pid, dev->cfg.sn, ub_platform_now());

    for (int i = 0; i < dev->sample_count; i++) {
        ub_report_payload_begin(&b, dev->sample_buf[i].ts);
        for (int f = 0; f < UB_MAX_FIELDS; f++) {
            if (dev->sample_buf[i].has_value[f]) {
                ub_report_add_field(&b, f + 1, dev->sample_buf[i].value[f]);
            }
        }
        ub_report_payload_end(&b);
    }

    int n = ub_report_end(&b);
    if (n < 0) {
        ub_platform_log("report buffer overflow, dropping buffered samples to recover");
        dev->sample_count = 0;
        return;
    }

    static char resp[512];
    int status;
    size_t resp_len;
    int rc = dev->transport->post(dev->transport->user_ctx, dev->cfg.server_host,
                                   dev->cfg.server_port, "/api/v1/data/report", buf, (size_t)n,
                                   &status, resp, sizeof(resp), &resp_len);
    if (rc != 0) {
        ub_platform_log("report transport error, will retry next cycle");
        return; /* buffered samples stay queued */
    }

    ub_report_response_t rr;
    if (ub_parse_report_response(resp, &rr) != 0) {
        ub_platform_log("malformed report response, will retry next cycle");
        return;
    }

    if (rr.c == UB_CODE_OK) {
        ub_platform_log("report accepted: %d sample(s)", dev->sample_count);
        if (rr.has_t) ub_platform_set_time(rr.t);
        dev->sample_count = 0;
        return;
    }

    /* Error path -- protocol §7. Every buffered sample is kept and retried
     * on the next cycle (bounded only by UB_SAMPLE_BUF_CAP, same as an
     * offline period); the one extra action taken here is proactively
     * re-syncing the clock on a timestamp rejection so the *next* attempt
     * has a better chance of landing inside the server's +-5 minute
     * window. */
    int code = rr.c;
    char msg[128];
    ub_parse_error_response(resp, &code, msg, sizeof(msg));
    ub_platform_log("report rejected (http=%d c=%d m=%s), keeping buffered samples for retry",
                     status, code, msg);

    if (code == UB_CODE_TIMESTAMP_OUT_OF_RANGE) {
        do_time_sync(dev);
    } else if (code == UB_CODE_DEVICE_DISABLED) {
        ub_platform_log("device disabled by admin -- will keep retrying in case it's re-enabled");
    }
}

/* ---- main tick -------------------------------------------------------------*/

void ub_device_tick(ub_device_t *dev) {
    uint64_t now_ms = ub_platform_monotonic_ms();

    if (dev->last_sample_ms == 0 ||
        now_ms - dev->last_sample_ms >= (uint64_t)dev->sample_interval_sec * 1000ULL) {
        sample(dev);
        dev->last_sample_ms = now_ms;
    }

    int should_report = dev->sample_count > 0 &&
                         (dev->last_report_ms == 0 ||
                          now_ms - dev->last_report_ms >= (uint64_t)dev->report_interval_sec * 1000ULL ||
                          dev->sample_count >= UB_SAMPLE_BUF_CAP);
    if (should_report) {
        do_report(dev);
        dev->last_report_ms = now_ms;
    }
}
