#include "ub_protocol.h"

#include <stdarg.h>
#include <stdio.h>
#include <string.h>

#include "ub_json.h"

/* ---- POST /api/v1/auth/time --------------------------------------------*/

int ub_build_time_request(const ub_identity_t *id, char *buf, size_t buf_cap) {
    int n = snprintf(buf, buf_cap, "{\"pid\":\"%s\",\"sn\":\"%s\"}", id->pid, id->sn);
    if (n < 0 || (size_t)n >= buf_cap) return -1;
    return n;
}

/* ---- POST /api/v1/data/report -------------------------------------------
 * `failed` is sticky: once one append doesn't fit (or an out-of-range field
 * number is requested), every later call becomes a no-op so ub_report_end()
 * can report a single clean -1 rather than the caller having to check every
 * intermediate call. */

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

void ub_report_begin(ub_report_builder_t *b, char *buf, size_t cap, const char *pid,
                      const char *sn, int64_t ts) {
    memset(b, 0, sizeof(*b));
    b->buf = buf;
    b->cap = cap;
    rb_append(b, "{\"pid\":\"%s\",\"sn\":\"%s\",\"ts\":%lld,\"payloads\":[", pid, sn,
              (long long)ts);
}

void ub_report_payload_begin(ub_report_builder_t *b, int64_t ts) {
    if (b->payload_count > 0) rb_append(b, ",");
    rb_append(b, "{\"ts\":%lld,\"feed\":{", (long long)ts);
    b->field_count = 0;
}

void ub_report_add_field(ub_report_builder_t *b, int field_no, double value) {
    if (field_no < 1 || field_no > UB_MAX_FIELDS) {
        b->failed = 1;
        return;
    }
    if (b->field_count > 0) rb_append(b, ",");
    rb_append(b, "\"field%d\":%g", field_no, value);
    b->field_count++;
}

void ub_report_payload_end(ub_report_builder_t *b) {
    rb_append(b, "}}");
    b->payload_count++;
}

int ub_report_end(ub_report_builder_t *b) {
    rb_append(b, "]}");
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
