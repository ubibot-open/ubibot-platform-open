/* Device state machine implementation. See ub_device.h for the module's
 * role in the overall architecture. Everything below only calls into
 * ub_protocol.h/ub_transport.h/ub_platform.h/ub_storage.h/ub_sensors.h, so
 * it compiles and runs unmodified once those five headers have a firmware
 * implementation. */
#include "ub_device.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "ub_hmac_sha256.h"
#include "ub_json.h"
#include "ub_platform.h"
#include "ub_sensors.h"
#include "ub_sha256.h"
#include "ub_storage.h"

#define UB_TOKEN_RENEW_MARGIN_SEC 300

/* Device-local nak reason codes for command results. These are diagnostic
 * codes the device makes up to explain *why* it naked a command -- they
 * don't need to match the server's protocol-level {"c":...} status codes
 * (those describe the HTTP round trip itself, not command outcomes). */
#define UB_DEV_ERR_UNKNOWN_CMD 1
#define UB_DEV_ERR_PROBE_TABLE_FULL 2
#define UB_DEV_ERR_PROBE_NOT_FOUND 3
#define UB_DEV_ERR_BAD_ARGS 4
#define UB_DEV_ERR_OTA_BUSY 5
#define UB_DEV_ERR_OTA_NOT_FOUND 6
#define UB_DEV_ERR_OTA_CANNOT_CANCEL 7
#define UB_DEV_ERR_OTA_CANCELLED 8
#define UB_DEV_ERR_OTA_HASH_MISMATCH 9
#define UB_DEV_ERR_OTA_DOWNLOAD_FAILED 10

#define UB_PERSIST_MAGIC 0x55424442u /* "UBDB" */
#define UB_PERSIST_VERSION 1

typedef struct {
    uint32_t magic;
    uint32_t version;

    char token[UB_TOKEN_MAX];
    int64_t token_expires_at;
    int has_clock_ref;

    int ci;
    int ui;
    int fe_count;
    char fe[UB_MAX_FE][UB_FE_NAME_MAX];

    ub_probe_t probes[UB_MAX_PROBES];

    ub_pending_result_t pending[UB_MAX_PENDING_ACKS];
    int pending_count;

    int ota_pending_self_check;
    char ota_cmd_id[UB_CMD_ID_MAX];
    char ota_version[UB_OTA_VERSION_MAX];
} ub_persisted_state_t;

static void queue_ack(ub_device_t *dev, const char *cmd_id);
static void queue_nak(ub_device_t *dev, const char *cmd_id, int code, const char *msg);
static void do_report(ub_device_t *dev);

/* ---- persistence --------------------------------------------------------*/

void ub_device_persist(ub_device_t *dev) {
    ub_persisted_state_t st;
    memset(&st, 0, sizeof(st));
    st.magic = UB_PERSIST_MAGIC;
    st.version = UB_PERSIST_VERSION;

    memcpy(st.token, dev->token, sizeof(st.token));
    st.token_expires_at = dev->token_expires_at;
    st.has_clock_ref = dev->has_clock_ref;

    st.ci = dev->ci;
    st.ui = dev->ui;
    st.fe_count = dev->fe_count;
    memcpy(st.fe, dev->fe, sizeof(st.fe));

    memcpy(st.probes, dev->probes, sizeof(st.probes));

    memcpy(st.pending, dev->pending, sizeof(st.pending));
    st.pending_count = dev->pending_count;

    if (dev->ota.state == UB_OTA_REBOOTING) {
        st.ota_pending_self_check = 1;
        memcpy(st.ota_cmd_id, dev->ota.cmd_id, sizeof(st.ota_cmd_id));
        memcpy(st.ota_version, dev->ota.version, sizeof(st.ota_version));
    }

    if (ub_storage_save("state", &st, sizeof(st)) != 0) {
        ub_platform_log("warning: failed to persist device state");
    }
}

void ub_device_init(ub_device_t *dev, const ub_device_config_t *cfg, const ub_transport_t *transport) {
    memset(dev, 0, sizeof(*dev));
    dev->cfg = *cfg;
    dev->transport = transport;
    dev->ci = 30;
    dev->ui = 300;

    ub_storage_set_base_dir(cfg->data_dir);

    ub_persisted_state_t st;
    int n = ub_storage_load("state", &st, sizeof(st));
    if (n == (int)sizeof(st) && st.magic == UB_PERSIST_MAGIC && st.version == UB_PERSIST_VERSION) {
        memcpy(dev->token, st.token, sizeof(dev->token));
        dev->token_expires_at = st.token_expires_at;
        dev->has_clock_ref = st.has_clock_ref;

        dev->ci = st.ci;
        dev->ui = st.ui;
        dev->fe_count = st.fe_count;
        memcpy(dev->fe, st.fe, sizeof(dev->fe));

        memcpy(dev->probes, st.probes, sizeof(dev->probes));

        memcpy(dev->pending, st.pending, sizeof(dev->pending));
        dev->pending_count = st.pending_count;

        if (st.ota_pending_self_check) {
            /* Simulated self-check: this build has no real firmware image
             * to validate, so it always "passes". A real target runs its
             * own startup diagnostics here and reports UB_OTA_ROLLED_BACK
             * (and naks the command) instead if they fail. */
            dev->ota.state = UB_OTA_SUCCESS;
            memcpy(dev->ota.cmd_id, st.ota_cmd_id, sizeof(dev->ota.cmd_id));
            memcpy(dev->ota.version, st.ota_version, sizeof(dev->ota.version));
            queue_ack(dev, dev->ota.cmd_id);
            ub_platform_log("post-reboot ota self-check passed (version %s)", dev->ota.version);
        }

        ub_platform_log("restored persisted state: activated=%d ci=%d ui=%d pending=%d",
                         dev->token[0] != '\0', dev->ci, dev->ui, dev->pending_count);
    } else {
        ub_platform_log("no persisted state found, starting fresh");
    }

    dev->running = 1;
}

/* ---- small helpers -------------------------------------------------------*/

static int fe_enabled(ub_device_t *dev, const char *name) {
    if (dev->fe_count == 0) return 1; /* no restriction received yet: sample everything */
    for (int i = 0; i < dev->fe_count; i++) {
        if (strcmp(dev->fe[i], name) == 0) return 1;
    }
    return 0;
}

static void queue_ack(ub_device_t *dev, const char *cmd_id) {
    if (dev->pending_count >= UB_MAX_PENDING_ACKS) {
        ub_platform_log("pending ack/nak queue full, dropping ack for %s", cmd_id);
        return;
    }
    ub_pending_result_t *p = &dev->pending[dev->pending_count++];
    memset(p, 0, sizeof(*p));
    snprintf(p->cmd_id, sizeof(p->cmd_id), "%s", cmd_id);
    p->is_nak = 0;
}

static void queue_nak(ub_device_t *dev, const char *cmd_id, int code, const char *msg) {
    if (dev->pending_count >= UB_MAX_PENDING_ACKS) {
        ub_platform_log("pending ack/nak queue full, dropping nak for %s", cmd_id);
        return;
    }
    ub_pending_result_t *p = &dev->pending[dev->pending_count++];
    memset(p, 0, sizeof(*p));
    snprintf(p->cmd_id, sizeof(p->cmd_id), "%s", cmd_id);
    p->is_nak = 1;
    p->code = code;
    snprintf(p->msg, sizeof(p->msg), "%s", msg ? msg : "");
}

static const char *ota_state_str(ub_ota_state_t s) {
    switch (s) {
        case UB_OTA_DOWNLOADING: return "downloading";
        case UB_OTA_VERIFYING: return "verifying";
        case UB_OTA_FLASHING: return "flashing";
        case UB_OTA_REBOOTING: return "rebooting";
        case UB_OTA_SUCCESS: return "success";
        case UB_OTA_FAILED: return "failed";
        case UB_OTA_ROLLED_BACK: return "rolled_back";
        default: return "idle";
    }
}

static int ota_progress_percent(const ub_ota_ctx_t *ota) {
    if (ota->total_size <= 0) return 0;
    long pct = (ota->downloaded * 100) / ota->total_size;
    if (pct > 100) pct = 100;
    if (pct < 0) pct = 0;
    return (int)pct;
}

/* Splits a URL from an ota command into host/port/path. No TLS support
 * (adequate for the local dev server and for a firmware target that talks
 * to a fixed, already-trusted host); falls back to the device's own
 * server for a bare path, since the firmware endpoint typically lives on
 * the same host as the rest of the API. */
static void parse_url(ub_device_t *dev, const char *url, char *host_out, size_t host_cap,
                       int *port_out, char *path_out, size_t path_cap) {
    const char *p = url;
    if (strncmp(p, "http://", 7) == 0) {
        p += 7;
    } else if (strncmp(p, "https://", 8) == 0) {
        p += 8;
    }

    if (p == url) {
        snprintf(host_out, host_cap, "%s", dev->cfg.server_host);
        *port_out = dev->cfg.server_port;
        snprintf(path_out, path_cap, "%s", url);
        return;
    }

    const char *slash = strchr(p, '/');
    char hostport[160];
    if (slash) {
        size_t hp_len = (size_t)(slash - p);
        if (hp_len >= sizeof(hostport)) hp_len = sizeof(hostport) - 1;
        memcpy(hostport, p, hp_len);
        hostport[hp_len] = '\0';
        snprintf(path_out, path_cap, "%s", slash);
    } else {
        snprintf(hostport, sizeof(hostport), "%s", p);
        snprintf(path_out, path_cap, "/");
    }

    char *colon = strchr(hostport, ':');
    if (colon) {
        *colon = '\0';
        *port_out = atoi(colon + 1);
        snprintf(host_out, host_cap, "%s", hostport);
    } else {
        snprintf(host_out, host_cap, "%s", hostport);
        *port_out = 80;
    }
}

/* ---- activation (protocol §4) --------------------------------------------*/

static int do_time_sync(ub_device_t *dev, int64_t *server_time, char *nonce, size_t nonce_cap) {
    ub_identity_t id = {dev->cfg.pid, dev->cfg.sn, dev->cfg.secret};
    char req[256];
    int n = ub_build_time_request(&id, req, sizeof(req));
    if (n < 0) return -1;

    char resp[512];
    int status;
    size_t resp_len;
    if (dev->transport->post(dev->transport->user_ctx, dev->cfg.server_host, dev->cfg.server_port,
                              "/api/v1/auth/time", req, (size_t)n, NULL, &status, resp,
                              sizeof(resp), &resp_len) != 0) {
        ub_platform_log("time sync: transport error");
        return -1;
    }

    ub_time_response_t tr;
    if (ub_parse_time_response(resp, &tr) != 0 || tr.c != UB_CODE_OK) {
        ub_platform_log("time sync rejected (http=%d c=%d)", status, tr.c);
        return -1;
    }
    *server_time = tr.t;
    snprintf(nonce, nonce_cap, "%s", tr.n);
    return 0;
}

static int do_activate_with(ub_device_t *dev, int64_t ts, const char *nonce) {
    ub_identity_t id = {dev->cfg.pid, dev->cfg.sn, dev->cfg.secret};
    char req[320];
    int n = ub_build_activate_request(&id, ts, nonce, req, sizeof(req));
    if (n < 0) return -1;

    char resp[512];
    int status;
    size_t resp_len;
    if (dev->transport->post(dev->transport->user_ctx, dev->cfg.server_host, dev->cfg.server_port,
                              "/api/v1/auth/activate", req, (size_t)n, NULL, &status, resp,
                              sizeof(resp), &resp_len) != 0) {
        ub_platform_log("activate: transport error");
        return -1;
    }

    ub_activate_response_t ar;
    if (ub_parse_activate_response(resp, &ar) != 0 || ar.c != UB_CODE_OK) {
        ub_platform_log("activate rejected (http=%d c=%d)", status, ar.c);
        return -1;
    }

    snprintf(dev->token, sizeof(dev->token), "%s", ar.token);
    dev->token_expires_at = ub_platform_now() + ar.exp;
    return 0;
}

static void activate(ub_device_t *dev) {
    if (dev->has_clock_ref) {
        if (do_activate_with(dev, ub_platform_now(), NULL) == 0) {
            ub_platform_log("warm-boot (local-clock) activation ok");
            ub_device_persist(dev);
            return;
        }
        ub_platform_log("warm-boot activation failed, falling back to time-sync path");
    }

    int64_t server_time;
    char nonce[UB_NONCE_MAX];
    if (do_time_sync(dev, &server_time, nonce, sizeof(nonce)) != 0) return;
    ub_platform_set_time(server_time);

    if (do_activate_with(dev, server_time, nonce) == 0) {
        dev->has_clock_ref = 1;
        ub_platform_log("cold-boot (nonce) activation ok");
        ub_device_persist(dev);
    }
}

/* ---- command dispatch (protocol §7) --------------------------------------*/

static void apply_set_cfg(ub_device_t *dev, const ub_cmd_t *cmd) {
    if (cmd->a_json) {
        const char *v;
        int64_t tmp;
        if ((v = ub_json_find_key(cmd->a_json, "ci")) != NULL && ub_json_get_int64(v, &tmp) == 0) {
            dev->ci = (int)tmp;
        }
        if ((v = ub_json_find_key(cmd->a_json, "ui")) != NULL && ub_json_get_int64(v, &tmp) == 0) {
            dev->ui = (int)tmp;
        }
    }
    queue_ack(dev, cmd->id);
}

static void apply_set_probe(ub_device_t *dev, const ub_cmd_t *cmd) {
    ub_set_probe_args_t args;
    if (ub_parse_set_probe_args(cmd->a_json, &args) != 0) {
        queue_nak(dev, cmd->id, UB_DEV_ERR_BAD_ARGS, "malformed set_probe args");
        return;
    }

    if (strcmp(args.op, "replace_all") == 0) {
        for (int i = 0; i < UB_MAX_PROBES; i++) dev->probes[i].in_use = 0;
        queue_ack(dev, cmd->id);
        dev->probes_dirty = 1;
        return;
    }

    if (strcmp(args.op, "remove") == 0) {
        int found = 0;
        for (int i = 0; i < UB_MAX_PROBES; i++) {
            if (dev->probes[i].in_use && strcmp(dev->probes[i].pid, args.pid) == 0) {
                dev->probes[i].in_use = 0;
                found = 1;
                break;
            }
        }
        if (found) {
            queue_ack(dev, cmd->id);
            dev->probes_dirty = 1;
        } else {
            queue_nak(dev, cmd->id, UB_DEV_ERR_PROBE_NOT_FOUND, "probe pid not found");
        }
        return;
    }

    if (strcmp(args.op, "upsert") == 0) {
        int slot = -1;
        for (int i = 0; i < UB_MAX_PROBES; i++) {
            if (dev->probes[i].in_use && strcmp(dev->probes[i].pid, args.pid) == 0) {
                slot = i;
                break;
            }
        }
        if (slot < 0) {
            for (int i = 0; i < UB_MAX_PROBES; i++) {
                if (!dev->probes[i].in_use) {
                    slot = i;
                    break;
                }
            }
        }
        if (slot < 0) {
            queue_nak(dev, cmd->id, UB_DEV_ERR_PROBE_TABLE_FULL, "probe table full");
            return;
        }

        ub_probe_t *p = &dev->probes[slot];
        int is_new = !p->in_use;
        snprintf(p->pid, sizeof(p->pid), "%s", args.pid);
        if (args.has_key) snprintf(p->key, sizeof(p->key), "%s", args.key);
        else if (is_new) snprintf(p->key, sizeof(p->key), "%s", args.pid);
        if (args.has_iface) snprintf(p->iface, sizeof(p->iface), "%s", args.iface);
        if (args.has_proto) snprintf(p->proto, sizeof(p->proto), "%s", args.proto);
        p->scale = args.has_scale ? args.scale : (is_new ? 1.0 : p->scale);
        p->offset = args.has_offset ? args.offset : (is_new ? 0.0 : p->offset);
        p->in_use = 1;

        queue_ack(dev, cmd->id);
        dev->probes_dirty = 1;
        return;
    }

    queue_nak(dev, cmd->id, UB_DEV_ERR_BAD_ARGS, "unknown set_probe op");
}

static void apply_ota_cmd(ub_device_t *dev, const ub_cmd_t *cmd) {
    ub_ota_args_t args;
    if (ub_parse_ota_args(cmd->a_json, &args) != 0) {
        queue_nak(dev, cmd->id, UB_DEV_ERR_BAD_ARGS, "malformed ota args");
        return;
    }

    if (strcmp(args.action, "start") == 0) {
        if (dev->ota.state != UB_OTA_IDLE) {
            queue_nak(dev, cmd->id, UB_DEV_ERR_OTA_BUSY, "ota already in progress");
            return;
        }
        memset(&dev->ota, 0, sizeof(dev->ota));
        dev->ota.state = UB_OTA_DOWNLOADING;
        snprintf(dev->ota.cmd_id, sizeof(dev->ota.cmd_id), "%s", cmd->id);
        snprintf(dev->ota.version, sizeof(dev->ota.version), "%s", args.version);
        snprintf(dev->ota.url, sizeof(dev->ota.url), "%s", args.url);
        dev->ota.total_size = args.has_size ? (long)args.size : 0;
        snprintf(dev->ota.sha256_hex, sizeof(dev->ota.sha256_hex), "%s", args.sha256_hex);
        ub_platform_log("ota start accepted: version=%s size=%ld", dev->ota.version,
                         dev->ota.total_size);
        /* Intentionally not acked yet: the ack/nak for this command is sent
         * once the whole OTA reaches a terminal state (success/failed/
         * rolled_back) via the post-reboot self-check handshake. */
        return;
    }

    if (strcmp(args.action, "cancel") == 0) {
        if (dev->ota.state == UB_OTA_IDLE) {
            queue_nak(dev, cmd->id, UB_DEV_ERR_OTA_NOT_FOUND, "no ota in progress");
            return;
        }
        if (dev->ota.state == UB_OTA_FLASHING || dev->ota.state == UB_OTA_REBOOTING) {
            queue_nak(dev, cmd->id, UB_DEV_ERR_OTA_CANNOT_CANCEL,
                      "flashing already started, cannot cancel");
            return;
        }
        queue_ack(dev, cmd->id);
        queue_nak(dev, dev->ota.cmd_id, UB_DEV_ERR_OTA_CANCELLED, "cancelled by server");
        dev->ota.cancel_requested = 1;
        dev->ota.state = UB_OTA_FAILED;
        return;
    }

    queue_nak(dev, cmd->id, UB_DEV_ERR_BAD_ARGS, "unknown ota action");
}

static void dispatch_cmds(ub_device_t *dev, const ub_cmd_t *cmds, int count) {
    for (int i = 0; i < count; i++) {
        const ub_cmd_t *cmd = &cmds[i];
        if (strcmp(cmd->tp, UB_CMD_TP_SET_CFG) == 0) {
            apply_set_cfg(dev, cmd);
        } else if (strcmp(cmd->tp, UB_CMD_TP_REBOOT) == 0) {
            queue_ack(dev, cmd->id);
            dev->reboot_requested = 1;
        } else if (strcmp(cmd->tp, UB_CMD_TP_CALIBRATE) == 0) {
            ub_platform_log("calibrate cmd %s (simulated, no-op)", cmd->id);
            queue_ack(dev, cmd->id);
        } else if (strcmp(cmd->tp, UB_CMD_TP_SET_PROBE) == 0) {
            apply_set_probe(dev, cmd);
        } else if (strcmp(cmd->tp, UB_CMD_TP_OTA) == 0) {
            apply_ota_cmd(dev, cmd);
        } else {
            queue_nak(dev, cmd->id, UB_DEV_ERR_UNKNOWN_CMD, "unknown command type");
        }
    }
}

/* ---- sampling & reporting (protocol §5) ----------------------------------*/

static void sample(ub_device_t *dev) {
    int cap = (int)(sizeof(dev->sample_buf) / sizeof(dev->sample_buf[0]));
    if (dev->sample_count >= cap) return; /* full: the next report will flush it */

    int idx = dev->sample_count;
    dev->sample_buf[idx].ts = ub_platform_now();
    dev->sample_buf[idx].has_temperature = 0;
    dev->sample_buf[idx].has_humidity = 0;

    if (fe_enabled(dev, "temperature")) {
        dev->sample_buf[idx].temperature = ub_sensor_read_temperature();
        dev->sample_buf[idx].has_temperature = 1;
    }
    if (fe_enabled(dev, "humidity")) {
        dev->sample_buf[idx].humidity = ub_sensor_read_humidity();
        dev->sample_buf[idx].has_humidity = 1;
    }
    dev->sample_count++;
}

static void do_report(ub_device_t *dev) {
    static char buf[4096];
    ub_report_builder_t b;
    ub_report_begin(&b, buf, sizeof(buf), dev->cfg.sn);

    for (int i = 0; i < dev->sample_count; i++) {
        ub_report_record_begin(&b, dev->sample_buf[i].ts);
        if (dev->sample_buf[i].has_temperature) {
            ub_report_add_field_f64(&b, "temperature", dev->sample_buf[i].temperature);
        }
        if (dev->sample_buf[i].has_humidity) {
            ub_report_add_field_f64(&b, "humidity", dev->sample_buf[i].humidity);
        }
        ub_report_record_end(&b);
    }

    int any_probe = 0;
    for (int i = 0; i < UB_MAX_PROBES; i++) {
        if (dev->probes[i].in_use) {
            any_probe = 1;
            break;
        }
    }
    if (any_probe) {
        ub_report_record_begin(&b, ub_platform_now());
        for (int i = 0; i < UB_MAX_PROBES; i++) {
            if (!dev->probes[i].in_use) continue;
            double v = ub_sensor_read_probe(dev->probes[i].pid, dev->probes[i].scale,
                                             dev->probes[i].offset);
            ub_report_add_field_f64(&b, dev->probes[i].key, v);
        }
        ub_report_record_end(&b);
    }

    for (int i = 0; i < dev->pending_count; i++) {
        if (dev->pending[i].is_nak) {
            ub_report_add_nak(&b, dev->pending[i].cmd_id, dev->pending[i].code, dev->pending[i].msg);
        } else {
            ub_report_add_ack(&b, dev->pending[i].cmd_id);
        }
    }

    if (dev->probes_dirty) {
        for (int i = 0; i < UB_MAX_PROBES; i++) {
            if (dev->probes[i].in_use) ub_report_add_prb(&b, dev->probes[i].pid);
        }
    }

    if (dev->ota.state != UB_OTA_IDLE) {
        ub_report_set_ota(&b, dev->ota.cmd_id, dev->ota.version, ota_state_str(dev->ota.state),
                           ota_progress_percent(&dev->ota));
    }

    int n = ub_report_end(&b);
    if (n < 0) {
        ub_platform_log("report buffer overflow, dropping buffered samples to recover");
        dev->sample_count = 0;
        return;
    }

    static char resp[4096];
    int status;
    size_t resp_len;
    int rc = dev->transport->post(dev->transport->user_ctx, dev->cfg.server_host,
                                   dev->cfg.server_port, "/api/v1/data/report", buf, (size_t)n,
                                   dev->token, &status, resp, sizeof(resp), &resp_len);
    if (rc != 0) {
        ub_platform_log("report transport error, will retry next cycle");
        return;
    }

    ub_report_response_t rr;
    if (ub_parse_report_response(resp, &rr) != 0) {
        ub_platform_log("malformed report response, will retry next cycle");
        return;
    }

    if (rr.c == UB_CODE_TOKEN_INVALID || rr.c == UB_CODE_TOKEN_EXPIRED) {
        ub_platform_log("report rejected: token invalid/expired, will reactivate");
        dev->token[0] = '\0';
        dev->token_expires_at = 0;
        return; /* buffered data stays queued and is retried after reactivation */
    }
    if (rr.c != UB_CODE_OK) {
        ub_platform_log("report rejected c=%d, dropping this batch", rr.c);
        dev->sample_count = 0;
        dev->pending_count = 0;
        dev->probes_dirty = 0;
        if (dev->ota.state == UB_OTA_SUCCESS || dev->ota.state == UB_OTA_FAILED ||
            dev->ota.state == UB_OTA_ROLLED_BACK) {
            memset(&dev->ota, 0, sizeof(dev->ota));
        }
        return;
    }

    /* Accepted: everything we sent is now the server's problem. */
    dev->sample_count = 0;
    dev->pending_count = 0;
    dev->probes_dirty = 0;
    if (dev->ota.state == UB_OTA_SUCCESS || dev->ota.state == UB_OTA_FAILED ||
        dev->ota.state == UB_OTA_ROLLED_BACK) {
        memset(&dev->ota, 0, sizeof(dev->ota));
    }

    if (rr.has_t) ub_platform_set_time(rr.t);
    if (rr.has_cfg) {
        dev->ci = rr.ci;
        dev->ui = rr.ui;
        dev->fe_count = rr.fe_count;
        memcpy(dev->fe, rr.fe, sizeof(dev->fe));
    }

    dispatch_cmds(dev, rr.cmd, rr.cmd_count);
    ub_device_persist(dev);
}

/* ---- OTA download (protocol §7.3) -----------------------------------------*/

typedef struct {
    ub_device_t *dev;
    FILE *f;
    ub_sha256_ctx_t sha;
    long next_report_at;
    int aborted;
} ota_chunk_ctx_t;

static int ota_on_chunk(void *ctx_, const uint8_t *data, size_t len) {
    ota_chunk_ctx_t *ctx = (ota_chunk_ctx_t *)ctx_;

    if (ctx->dev->ota.cancel_requested) {
        ctx->aborted = 1;
        return -1;
    }

    if (ctx->f) fwrite(data, 1, len, ctx->f);
    ub_sha256_update(&ctx->sha, data, len);
    ctx->dev->ota.downloaded += (long)len;

    if (ctx->dev->ota.total_size > 0 && ctx->dev->ota.downloaded >= ctx->next_report_at) {
        do_report(ctx->dev); /* periodic progress ping; also lets a mid-download cancel arrive */
        long step = ctx->dev->ota.total_size / 10;
        if (step <= 0) step = 1;
        ctx->next_report_at = ctx->dev->ota.downloaded + step;
    }
    return 0;
}

static void finish_ota_failed(ub_device_t *dev, int code, const char *msg) {
    queue_nak(dev, dev->ota.cmd_id, code, msg);
    dev->ota.state = UB_OTA_FAILED;
    do_report(dev);
}

static void run_ota_download(ub_device_t *dev) {
    do_report(dev); /* flush the initial "downloading" 0% status before the blocking transfer */

    char path[UB_DEV_DIR_MAX + 32];
    snprintf(path, sizeof(path), "%s/ota_download.bin", dev->cfg.data_dir);

    ota_chunk_ctx_t ctx;
    memset(&ctx, 0, sizeof(ctx));
    ctx.dev = dev;
    ctx.f = fopen(path, "wb");
    ub_sha256_init(&ctx.sha);
    ctx.next_report_at = dev->ota.total_size > 0 ? dev->ota.total_size / 10 : (10L * 1024 * 1024);

    char host[UB_DEV_HOST_MAX];
    int port;
    char url_path[192];
    parse_url(dev, dev->ota.url, host, sizeof(host), &port, url_path, sizeof(url_path));

    int status;
    long content_length;
    int rc = dev->transport->download(dev->transport->user_ctx, host, port, url_path, dev->token,
                                       0, &status, &content_length, ota_on_chunk, &ctx);

    if (ctx.f) fclose(ctx.f);

    if (ctx.aborted) {
        ub_platform_log("ota download aborted (cancelled)");
        return; /* the cancel handler already queued the nak and set state=FAILED */
    }
    if (rc != 0 || status / 100 != 2) {
        ub_platform_log("ota download failed (rc=%d http=%d)", rc, status);
        finish_ota_failed(dev, UB_DEV_ERR_OTA_DOWNLOAD_FAILED, "download failed");
        return;
    }

    uint8_t digest[UB_SHA256_DIGEST_LEN];
    char digest_hex[65];
    ub_sha256_final(&ctx.sha, digest);
    ub_hex_encode(digest, sizeof(digest), digest_hex);

    dev->ota.state = UB_OTA_VERIFYING;
    do_report(dev);

    if (dev->ota.sha256_hex[0] != '\0' && strcmp(digest_hex, dev->ota.sha256_hex) != 0) {
        ub_platform_log("ota sha256 mismatch: got %s want %s", digest_hex, dev->ota.sha256_hex);
        finish_ota_failed(dev, UB_DEV_ERR_OTA_HASH_MISMATCH, "sha256 mismatch");
        return;
    }

    dev->ota.state = UB_OTA_FLASHING;
    do_report(dev);
    ub_platform_log("ota: simulated flash write of %ld bytes (version %s)", dev->ota.downloaded,
                     dev->ota.version);

    dev->ota.state = UB_OTA_REBOOTING;
    ub_device_persist(dev); /* records ota_pending_self_check so the next process start resumes it */
    do_report(dev);

    ub_platform_reboot(); /* host: exits the process here; firmware: real MCU reset */
}

/* ---- main tick -------------------------------------------------------------*/

void ub_device_tick(ub_device_t *dev) {
    uint64_t now_ms = ub_platform_monotonic_ms();

    int64_t token_remaining = dev->token[0] ? dev->token_expires_at - ub_platform_now() : -1;
    if (token_remaining < UB_TOKEN_RENEW_MARGIN_SEC) {
        activate(dev);
        return;
    }

    if (dev->last_sample_ms == 0 || now_ms - dev->last_sample_ms >= (uint64_t)dev->ci * 1000ULL) {
        sample(dev);
        dev->last_sample_ms = now_ms;
    }

    int cap = (int)(sizeof(dev->sample_buf) / sizeof(dev->sample_buf[0]));
    int should_report = dev->last_report_ms == 0 ||
                         now_ms - dev->last_report_ms >= (uint64_t)dev->ui * 1000ULL ||
                         dev->pending_count > 0 || dev->probes_dirty || dev->sample_count >= cap;
    if (should_report) {
        do_report(dev);
        dev->last_report_ms = now_ms;
    }

    if (dev->reboot_requested) {
        ub_device_persist(dev);
        ub_platform_reboot();
    }

    if (dev->ota.state == UB_OTA_DOWNLOADING && dev->ota.downloaded == 0) {
        run_ota_download(dev);
    }
}
