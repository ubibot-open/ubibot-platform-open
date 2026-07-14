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
 * caller having to check every intermediate call. */

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

void ub_report_begin(ub_report_builder_t *b, char *buf, size_t cap, const char *did) {
    memset(b, 0, sizeof(*b));
    b->buf = buf;
    b->cap = cap;
    rb_append(b, "{\"did\":\"%s\",\"recs\":[", did);
}

void ub_report_record_begin(ub_report_builder_t *b, int64_t ts) {
    if (b->rec_count > 0) rb_append(b, ",");
    rb_append(b, "{\"ts\":%lld,\"d\":{", (long long)ts);
    b->in_record = 1;
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
    b->in_record = 0;
    b->rec_count++;
}

void ub_report_add_ack(ub_report_builder_t *b, const char *cmd_id) {
    if (!b->recs_closed) {
        rb_append(b, "]");
        b->recs_closed = 1;
        rb_append(b, ",\"ack\":[");
    } else if (b->ack_count > 0) {
        rb_append(b, ",");
    }
    rb_append(b, "\"%s\"", cmd_id);
    b->ack_count++;
}

int ub_report_end(ub_report_builder_t *b) {
    if (!b->recs_closed) {
        rb_append(b, "]");
        b->recs_closed = 1;
    } else if (b->ack_count > 0) {
        rb_append(b, "]");
    }
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
        const char *civ = ub_json_find_key(v, "ci");
        const char *uiv = ub_json_find_key(v, "ui");
        int64_t ci_val = 0, ui_val = 0;
        if (civ && ub_json_get_int64(civ, &ci_val) == 0 && uiv &&
            ub_json_get_int64(uiv, &ui_val) == 0) {
            out->has_cfg = 1;
            out->ci = (int)ci_val;
            out->ui = (int)ui_val;
        }
    }

    if ((v = ub_json_find_key(json, "cmd")) != NULL && ub_json_is_array(v)) {
        const char *elem = ub_json_array_first(v);
        while (elem && out->cmd_count < UB_MAX_CMDS) {
            ub_cmd_t *item = &out->cmd[out->cmd_count];
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

            out->cmd_count++;
            elem = ub_json_array_next(elem);
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
