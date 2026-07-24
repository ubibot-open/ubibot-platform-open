/* Device-side codec for the UbiBot HTTP protocol
 * (docs/UbiBot开放平台硬件通信协议.md).
 *
 * This module only builds request bodies and parses response bodies; it
 * does not do any I/O (see ub_transport.h for that seam). Every function
 * writes into caller-supplied, fixed-size buffers — nothing here calls
 * malloc, which keeps the memory footprint predictable on an MCU.
 *
 * The protocol itself is deliberately tiny: a device identifies itself with
 * a plaintext pid+sn (no secret, no signature, no session token, no
 * activation step) and talks to exactly two endpoints — a clock-reference
 * convenience (/auth/time) and a batched data upload (/data/report). There
 * is no server-to-device command channel of any kind. */
#ifndef UB_PROTOCOL_H
#define UB_PROTOCOL_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Highest allowed field number in a payload's "feed" object (protocol §5:
 * field1..field20, field1/2/3 default to temperature/humidity/light). */
#define UB_MAX_FIELDS 20

/* Business status codes (the "c" field), per protocol §7. */
#define UB_CODE_OK 0
#define UB_CODE_TIMESTAMP_OUT_OF_RANGE 1002
#define UB_CODE_MALFORMED_BODY 1003
#define UB_CODE_DEVICE_DISABLED 1103
#define UB_CODE_RATE_LIMITED 1900
#define UB_CODE_SERVER_ERROR 5000

typedef struct {
    const char *pid;
    const char *sn;
} ub_identity_t;

/* ---- POST /api/v1/auth/time -------------------------------------------
 * No auth, no signature: this is purely a clock-reference convenience for
 * a device with no RTC. Returns the number of bytes written (excluding the
 * NUL terminator), or -1 if buf_cap was too small. */
int ub_build_time_request(const ub_identity_t *id, char *buf, size_t buf_cap);

typedef struct {
    int c;
    int64_t t; /* server's current unix time, seconds */
} ub_time_response_t;

int ub_parse_time_response(const char *json, ub_time_response_t *out);

/* ---- POST /api/v1/data/report ------------------------------------------
 * Streaming builder writing directly into the caller's buffer; no
 * intermediate allocation or data structure. Call order matters (it's a
 * one-pass writer):
 *
 *   ub_report_begin(pid, sn, ts)
 *   N x (ub_report_payload_begin(ts) -> M x ub_report_add_field -> ub_report_payload_end())
 *   ub_report_end()
 *
 * Produces exactly the shape in protocol §4:
 *   {"pid":"...","sn":"...","ts":<ts>,"payloads":[{"ts":<ts>,"feed":{"field1":...}}]}
 */
typedef struct {
    char *buf;
    size_t cap;
    size_t len;
    int failed;

    int payload_count;
    int field_count;
} ub_report_builder_t;

void ub_report_begin(ub_report_builder_t *b, char *buf, size_t cap, const char *pid,
                      const char *sn, int64_t ts);
void ub_report_payload_begin(ub_report_builder_t *b, int64_t ts);
/* field_no must be in [1, UB_MAX_FIELDS]; out-of-range values are dropped
 * (sticky-failed) rather than silently writing a bogus key. */
void ub_report_add_field(ub_report_builder_t *b, int field_no, double value);
void ub_report_payload_end(ub_report_builder_t *b);

/* Returns total length written, or -1 if the buffer was too small at any
 * step (failure is sticky — check this return value, not each add_* call). */
int ub_report_end(ub_report_builder_t *b);

typedef struct {
    int c;
    int64_t t; /* server's current unix time, seconds; device may use it to correct its own clock */
    int has_t;
} ub_report_response_t;

int ub_parse_report_response(const char *json, ub_report_response_t *out);

/* ---- shared error envelope ----------------------------------------------
 * Generic {"c":N,"m":"..."} shape, used by every endpoint when c != 0. */
int ub_parse_error_response(const char *json, int *code, char *msg, size_t msg_cap);

#ifdef __cplusplus
}
#endif

#endif /* UB_PROTOCOL_H */
