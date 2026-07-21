#include "ub_protocol.h"

#include <stdarg.h>
#include <stdio.h>
#include <string.h>

#include "ub_hmac_sha256.h"
#include "ub_json.h"

/* ---- signing ----------------------------------------------------------- */

void ub_sign_time(const ub_identity_t *id, char sign_hex[UB_SIGN_HEX_LEN + 1]) {
    ub_hmac_sha256_ctx_t ctx;
    ub_hmac_sha256_init(&ctx, (const uint8_t *)id->secret, strlen(id->secret));
    ub_hmac_sha256_update(&ctx, (const uint8_t *)id->pid, strlen(id->pid));
    ub_hmac_sha256_update(&ctx, (const uint8_t *)id->sn, strlen(id->sn));

    uint8_t digest[UB_SHA256_DIGEST_LEN];
    ub_hmac_sha256_final(&ctx, digest);
    ub_hex_encode(digest, sizeof(digest), sign_hex);
}

void ub_sign_activate(const ub_identity_t *id, int64_t ts, const char *n,
                       char sign_hex[UB_SIGN_HEX_LEN + 1]) {
    if (!n) n = "";
    char ts_str[24];
    snprintf(ts_str, sizeof(ts_str), "%lld", (long long)ts);

    ub_hmac_sha256_ctx_t ctx;
    ub_hmac_sha256_init(&ctx, (const uint8_t *)id->secret, strlen(id->secret));
    ub_hmac_sha256_update(&ctx, (const uint8_t *)id->pid, strlen(id->pid));
    ub_hmac_sha256_update(&ctx, (const uint8_t *)id->sn, strlen(id->sn));
    ub_hmac_sha256_update(&ctx, (const uint8_t *)ts_str, strlen(ts_str));
    ub_hmac_sha256_update(&ctx, (const uint8_t *)n, strlen(n));

    uint8_t digest[UB_SHA256_DIGEST_LEN];
    ub_hmac_sha256_final(&ctx, digest);
    ub_hex_encode(digest, sizeof(digest), sign_hex);
}

/* ---- request builders ---------------------------------------------------*/

int ub_build_time_request(const ub_identity_t *id, char *buf, size_t buf_cap) {
    char sign[UB_SIGN_HEX_LEN + 1];
    ub_sign_time(id, sign);

    int n = snprintf(buf, buf_cap, "{\"pid\":\"%s\",\"sn\":\"%s\",\"sign\":\"%s\"}", id->pid,
                      id->sn, sign);
    if (n < 0 || (size_t)n >= buf_cap) return -1;
    return n;
}

int ub_build_activate_request(const ub_identity_t *id, int64_t ts, const char *nonce, char *buf,
                               size_t buf_cap) {
    char sign[UB_SIGN_HEX_LEN + 1];
    ub_sign_activate(id, ts, nonce, sign);

    int n;
    if (nonce && nonce[0] != '\0') {
        n = snprintf(buf, buf_cap,
                     "{\"pid\":\"%s\",\"sn\":\"%s\",\"ts\":%lld,\"n\":\"%s\",\"sign\":\"%s\"}",
                     id->pid, id->sn, (long long)ts, nonce, sign);
    } else {
        n = snprintf(buf, buf_cap, "{\"pid\":\"%s\",\"sn\":\"%s\",\"ts\":%lld,\"sign\":\"%s\"}",
                     id->pid, id->sn, (long long)ts, sign);
    }
    if (n < 0 || (size_t)n >= buf_cap) return -1;
    return n;
}

/* ---- data report builder -------------------------------------------------
 * `failed` is sticky: once one append doesn't fit, every later call becomes
 * a no-op so ub_report_end() can report a single clean -1 rather than the
 * caller having to check every intermediate call.
 *
 * Sections (recs/ack/nak/prb/ota) are emitted in that fixed order
 * regardless of which ones the caller actually uses; each add_* call for a
 * later section first closes every section before it that's still open, via
 * the close_through_* cascade below. This keeps the writer single-pass
 * (no seeking back to patch a bracket) while letting every section be
 * fully optional. */

static void rb_append(ub_report_builder_t *b, const char *fmt, ...) {
    if (b->failed) return;

    va_list ap;
    va_start(ap, fmt);
    int n = vsnprintf(b->buf + b->len, b->cap - b->len, fmt, ap);
    va_end(ap);

    if (n < 0 || (size_t)n >= b->cap - b->len) {
        b->failed = 1;
        return;
    }
    b->len += (size_t)n;
}

static void close_recs(ub_report_builder_t *b) {
    if (!b->recs_closed) {
        rb_append(b, "]");
        b->recs_closed = 1;
    }
}

static void close_ack(ub_report_builder_t *b) {
    close_recs(b);
    if (b->ack_count > 0 && !b->ack_closed) {
        rb_append(b, "]");
        b->ack_closed = 1;
    }
}

static void close_nak(ub_report_builder_t *b) {
    close_ack(b);
    if (b->nak_count > 0 && !b->nak_closed) {
        rb_append(b, "]");
        b->nak_closed = 1;
    }
}

static void close_prb(ub_report_builder_t *b) {
    close_nak(b);
    if (b->prb_count > 0 && !b->prb_closed) {
        rb_append(b, "]");
        b->prb_closed = 1;
    }
}

void ub_report_begin(ub_report_builder_t *b, char *buf, size_t cap, const char *did) {
    memset(b, 0, sizeof(*b));
    b->buf = buf;
    b->cap = cap;
    rb_append(b, "{\"did\":\"%s\",\"recs\":[", did);
}

void ub_report_record_begin(ub_report_builder_t *b, int64_t ts) {
    if (b->rec_count > 0) rb_append(b, ",");
    rb_append(b, "{\"ts\":%lld,\"d\":{", (long long)ts);
    b->field_count = 0;
}

void ub_report_add_field_i64(ub_report_builder_t *b, const char *name, int64_t value) {
    if (b->field_count > 0) rb_append(b, ",");
    rb_append(b, "\"%s\":%lld", name, (long long)value);
    b->field_count++;
}

void ub_report_add_field_f64(ub_report_builder_t *b, const char *name, double value) {
    if (b->field_count > 0) rb_append(b, ",");
    rb_append(b, "\"%s\":%g", name, value);
    b->field_count++;
}

void ub_report_add_field_raw(ub_report_builder_t *b, const char *name, const char *raw_json) {
    if (b->field_count > 0) rb_append(b, ",");
    rb_append(b, "\"%s\":%s", name, raw_json);
    b->field_count++;
}

void ub_report_record_end(ub_report_builder_t *b) {
    rb_append(b, "}}");
    b->rec_count++;
}

void ub_report_add_ack(ub_report_builder_t *b, const char *cmd_id) {
    close_recs(b);
    if (b->ack_count == 0) {
        rb_append(b, ",\"ack\":[");
    } else {
        rb_append(b, ",");
    }
    rb_append(b, "\"%s\"", cmd_id);
    b->ack_count++;
}

void ub_report_add_nak(ub_report_builder_t *b, const char *cmd_id, int code, const char *msg) {
    close_ack(b);
    if (b->nak_count == 0) {
        rb_append(b, ",\"nak\":[");
    } else {
        rb_append(b, ",");
    }
    rb_append(b, "{\"id\":\"%s\",\"c\":%d,\"m\":\"%s\"}", cmd_id, code, msg ? msg : "");
    b->nak_count++;
}

void ub_report_add_prb(ub_report_builder_t *b, const char *pid) {
    close_nak(b);
    if (b->prb_count == 0) {
        rb_append(b, ",\"prb\":[");
    } else {
        rb_append(b, ",");
    }
    rb_append(b, "\"%s\"", pid);
    b->prb_count++;
}

void ub_report_set_ota(ub_report_builder_t *b, const char *cmd_id, const char *version,
                        const char *state, int progress) {
    close_prb(b);
    rb_append(b, ",\"ota\":{\"id\":\"%s\",\"version\":\"%s\",\"state\":\"%s\",\"progress\":%d}",
               cmd_id, version ? version : "", state, progress);
}

int ub_report_end(ub_report_builder_t *b) {
    close_prb(b);
    rb_append(b, "}");

    if (b->failed) return -1;
    return (int)b->len;
}

/* ---- response parsing ----------------------------------------------------*/

static int read_code(const char *json, int *code) {
    const char *v = ub_json_find_key(json, "c");
    int64_t c;
    if (!v || ub_json_get_int64(v, &c) != 0) return -1;
    *code = (int)c;
    return 0;
}

/* Shared by ub_parse_report_response and ub_parse_poll_response: both carry
 * an identical "cfg":{"ci":N,"ui":N,"fe":[...]} block. */
static void parse_cfg_block(const char *cfg_obj, int *has_cfg, int *ci, int *ui, char fe[][UB_FE_NAME_MAX],
                             int *fe_count) {
    const char *civ = ub_json_find_key(cfg_obj, "ci");
    const char *uiv = ub_json_find_key(cfg_obj, "ui");
    int64_t ci_val = 0, ui_val = 0;
    if (!civ || ub_json_get_int64(civ, &ci_val) != 0 || !uiv ||
        ub_json_get_int64(uiv, &ui_val) != 0) {
        return;
    }
    *has_cfg = 1;
    *ci = (int)ci_val;
    *ui = (int)ui_val;

    *fe_count = 0;
    const char *fev = ub_json_find_key(cfg_obj, "fe");
    if (fev && ub_json_is_array(fev)) {
        const char *elem = ub_json_array_first(fev);
        while (elem && *fe_count < UB_MAX_FE) {
            if (ub_json_get_string(elem, fe[*fe_count], UB_FE_NAME_MAX) == 0) {
                (*fe_count)++;
            }
            elem = ub_json_array_next(elem);
        }
    }
}

/* Shared by ub_parse_report_response and ub_parse_poll_response: both carry
 * an identical "cmd":[{"id":..,"tp":..,"a":{...}}] queue. */
static void parse_cmd_array(const char *json, ub_cmd_t cmd[], int *cmd_count) {
    *cmd_count = 0;
    const char *v = ub_json_find_key(json, "cmd");
    if (!v || !ub_json_is_array(v)) return;

    const char *elem = ub_json_array_first(v);
    while (elem && *cmd_count < UB_MAX_CMDS) {
        ub_cmd_t *item = &cmd[*cmd_count];
        memset(item, 0, sizeof(*item));

        const char *idv = ub_json_find_key(elem, "id");
        const char *tpv = ub_json_find_key(elem, "tp");
        if (idv) ub_json_get_string(idv, item->id, sizeof(item->id));
        if (tpv) ub_json_get_string(tpv, item->tp, sizeof(item->tp));

        const char *av = ub_json_find_key(elem, "a");
        if (av && ub_json_is_object(av)) {
            const char *aend = ub_json_skip_value(av);
            if (aend) {
                item->a_json = av;
                item->a_json_len = (size_t)(aend - av);
            }
        }

        (*cmd_count)++;
        elem = ub_json_array_next(elem);
    }
}

int ub_parse_time_response(const char *json, ub_time_response_t *out) {
    memset(out, 0, sizeof(*out));
    if (read_code(json, &out->c) != 0) return -1;
    if (out->c != UB_CODE_OK) return 0;

    const char *v;
    if (!(v = ub_json_find_key(json, "t")) || ub_json_get_int64(v, &out->t) != 0) return -1;
    if (!(v = ub_json_find_key(json, "n")) || ub_json_get_string(v, out->n, sizeof(out->n)) != 0)
        return -1;
    return 0;
}

int ub_parse_activate_response(const char *json, ub_activate_response_t *out) {
    memset(out, 0, sizeof(*out));
    if (read_code(json, &out->c) != 0) return -1;
    if (out->c != UB_CODE_OK) return 0;

    const char *v;
    if (!(v = ub_json_find_key(json, "token")) ||
        ub_json_get_string(v, out->token, sizeof(out->token)) != 0)
        return -1;
    if (!(v = ub_json_find_key(json, "exp")) || ub_json_get_int64(v, &out->exp) != 0) return -1;
    return 0;
}

int ub_parse_report_response(const char *json, ub_report_response_t *out) {
    memset(out, 0, sizeof(*out));
    if (read_code(json, &out->c) != 0) return -1;
    if (out->c != UB_CODE_OK) return 0;

    const char *v;
    if ((v = ub_json_find_key(json, "t")) != NULL && ub_json_get_int64(v, &out->t) == 0) {
        out->has_t = 1;
    }

    if ((v = ub_json_find_key(json, "cfg")) != NULL && ub_json_is_object(v)) {
        parse_cfg_block(v, &out->has_cfg, &out->ci, &out->ui, out->fe, &out->fe_count);
    }

    parse_cmd_array(json, out->cmd, &out->cmd_count);
    return 0;
}

int ub_parse_poll_response(const char *json, ub_poll_response_t *out) {
    memset(out, 0, sizeof(*out));
    if (read_code(json, &out->c) != 0) return -1;
    if (out->c != UB_CODE_OK) return 0;

    const char *v;
    if ((v = ub_json_find_key(json, "cfg")) != NULL && ub_json_is_object(v)) {
        parse_cfg_block(v, &out->has_cfg, &out->ci, &out->ui, out->fe, &out->fe_count);
    }

    parse_cmd_array(json, out->cmd, &out->cmd_count);
    return 0;
}

int ub_parse_set_probe_args(const char *a_json, ub_set_probe_args_t *out) {
    memset(out, 0, sizeof(*out));
    if (!a_json || !ub_json_is_object(a_json)) return -1;

    const char *v;
    if (!(v = ub_json_find_key(a_json, "op")) || ub_json_get_string(v, out->op, sizeof(out->op)) != 0)
        return -1;
    if (!(v = ub_json_find_key(a_json, "pid")) ||
        ub_json_get_string(v, out->pid, sizeof(out->pid)) != 0)
        return -1;

    if ((v = ub_json_find_key(a_json, "key")) != NULL)
        out->has_key = (ub_json_get_string(v, out->key, sizeof(out->key)) == 0);
    if ((v = ub_json_find_key(a_json, "iface")) != NULL)
        out->has_iface = (ub_json_get_string(v, out->iface, sizeof(out->iface)) == 0);
    if ((v = ub_json_find_key(a_json, "proto")) != NULL)
        out->has_proto = (ub_json_get_string(v, out->proto, sizeof(out->proto)) == 0);
    if ((v = ub_json_find_key(a_json, "addr")) != NULL)
        out->has_addr = (ub_json_get_string(v, out->addr, sizeof(out->addr)) == 0);
    if ((v = ub_json_find_key(a_json, "dtype")) != NULL)
        out->has_dtype = (ub_json_get_string(v, out->dtype, sizeof(out->dtype)) == 0);
    if ((v = ub_json_find_key(a_json, "byte_order")) != NULL)
        out->has_byte_order = (ub_json_get_string(v, out->byte_order, sizeof(out->byte_order)) == 0);

    int64_t tmp;
    if ((v = ub_json_find_key(a_json, "fc")) != NULL && ub_json_get_int64(v, &tmp) == 0) {
        out->fc = (int)tmp;
        out->has_fc = 1;
    }
    if ((v = ub_json_find_key(a_json, "reg")) != NULL && ub_json_get_int64(v, &tmp) == 0) {
        out->reg = (int)tmp;
        out->has_reg = 1;
    }
    if ((v = ub_json_find_key(a_json, "cnt")) != NULL && ub_json_get_int64(v, &tmp) == 0) {
        out->cnt = (int)tmp;
        out->has_cnt = 1;
    }
    if ((v = ub_json_find_key(a_json, "ci")) != NULL && ub_json_get_int64(v, &tmp) == 0) {
        out->ci = (int)tmp;
        out->has_ci = 1;
    }
    if ((v = ub_json_find_key(a_json, "timeout")) != NULL && ub_json_get_int64(v, &tmp) == 0) {
        out->timeout = (int)tmp;
        out->has_timeout = 1;
    }
    if ((v = ub_json_find_key(a_json, "retry")) != NULL && ub_json_get_int64(v, &tmp) == 0) {
        out->retry = (int)tmp;
        out->has_retry = 1;
    }

    double dtmp;
    if ((v = ub_json_find_key(a_json, "scale")) != NULL && ub_json_get_double(v, &dtmp) == 0) {
        out->scale = dtmp;
        out->has_scale = 1;
    }
    if ((v = ub_json_find_key(a_json, "offset")) != NULL && ub_json_get_double(v, &dtmp) == 0) {
        out->offset = dtmp;
        out->has_offset = 1;
    }

    return 0;
}

int ub_parse_ota_args(const char *a_json, ub_ota_args_t *out) {
    memset(out, 0, sizeof(*out));
    if (!a_json || !ub_json_is_object(a_json)) return -1;

    const char *v;
    if (!(v = ub_json_find_key(a_json, "action")) ||
        ub_json_get_string(v, out->action, sizeof(out->action)) != 0)
        return -1;

    /* "cancel" only needs action (matched against the in-flight OTA by the
     * caller); everything else is optional so a cancel body of just
     * {"action":"cancel"} parses cleanly. */
    if ((v = ub_json_find_key(a_json, "version")) != NULL)
        ub_json_get_string(v, out->version, sizeof(out->version));
    if ((v = ub_json_find_key(a_json, "url")) != NULL)
        ub_json_get_string(v, out->url, sizeof(out->url));
    if ((v = ub_json_find_key(a_json, "sha256")) != NULL)
        ub_json_get_string(v, out->sha256_hex, sizeof(out->sha256_hex));

    if ((v = ub_json_find_key(a_json, "size")) != NULL && ub_json_get_int64(v, &out->size) == 0) {
        out->has_size = 1;
    }
    if ((v = ub_json_find_key(a_json, "sig")) != NULL &&
        ub_json_get_string(v, out->sig, sizeof(out->sig)) == 0) {
        out->has_sig = 1;
    }
    if ((v = ub_json_find_key(a_json, "force")) != NULL) {
        int b;
        if (ub_json_get_bool(v, &b) == 0) {
            out->force = b;
            out->has_force = 1;
        }
    }

    return 0;
}

int ub_parse_error_response(const char *json, int *code, char *msg, size_t msg_cap) {
    if (read_code(json, code) != 0) return -1;

    const char *v = ub_json_find_key(json, "m");
    if (v) {
        ub_json_get_string(v, msg, msg_cap);
    } else if (msg_cap > 0) {
        msg[0] = '\0';
    }
    return 0;
}
