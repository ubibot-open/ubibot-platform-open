/* Device-side codec for the UbiBot HTTP protocol
 * (docs/UbiBot开放平台硬件通信协议.docx).
 *
 * This module only builds request bodies and parses response bodies; it
 * does not do any I/O (see ub_transport.h for that seam). Every function
 * writes into caller-supplied, fixed-size buffers — nothing here calls
 * malloc, which keeps the memory footprint predictable on an MCU. */
#ifndef UB_PROTOCOL_H
#define UB_PROTOCOL_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define UB_SIGN_HEX_LEN 64 /* HMAC-SHA256 hex string, no NUL */
#define UB_TOKEN_MAX 65    /* 64 hex chars + NUL */
#define UB_NONCE_MAX 32
#define UB_CMD_ID_MAX 24
#define UB_CMD_TP_MAX 24

/* Business status codes (the "c" field), mirroring internal/protocol on the
 * server. */
#define UB_CODE_OK 0
#define UB_CODE_SIGN_MISMATCH 1002
#define UB_CODE_MALFORMED_BODY 1003
#define UB_CODE_TOKEN_INVALID 1101
#define UB_CODE_TOKEN_EXPIRED 1102
#define UB_CODE_DEVICE_NOT_FOUND 1103
#define UB_CODE_RATE_LIMITED 1900
#define UB_CODE_SERVER_ERROR 5000

typedef struct {
    const char *pid;
    const char *sn;
    const char *secret; /* never serialized; used only to compute signatures */
} ub_identity_t;

/* ---- signing --------------------------------------------------------- */

/* HEX(HMAC-SHA256(secret, pid+sn)), for /auth/time. */
void ub_sign_time(const ub_identity_t *id, char sign_hex[UB_SIGN_HEX_LEN + 1]);

/* HEX(HMAC-SHA256(secret, pid+sn+ts+n)) for /auth/activate. Pass n="" (not
 * NULL) when the device is signing with its own clock and no nonce — both
 * activation paths share this one formula. */
void ub_sign_activate(const ub_identity_t *id, int64_t ts, const char *n,
                       char sign_hex[UB_SIGN_HEX_LEN + 1]);

/* ---- request builders -------------------------------------------------
 * Each returns the number of bytes written (excluding the NUL terminator),
 * or -1 if buf_cap was too small. */

int ub_build_time_request(const ub_identity_t *id, char *buf, size_t buf_cap);

/* nonce may be NULL or "" for the local-clock path. */
int ub_build_activate_request(const ub_identity_t *id, int64_t ts, const char *nonce,
                               char *buf, size_t buf_cap);

/* ---- data report builder ----------------------------------------------
 * Streaming builder: begin() -> N x (record_begin -> M x add_field ->
 * record_end) -> [add_ack]* -> end(). Writes directly into the caller's
 * buffer; no intermediate allocation or data structure.
 */
typedef struct {
    char *buf;
    size_t cap;
    size_t len;
    int failed;
    int rec_count;
    int field_count;
    int ack_count;
    int in_record;
    int recs_closed;
} ub_report_builder_t;

void ub_report_begin(ub_report_builder_t *b, char *buf, size_t cap, const char *did);
void ub_report_record_begin(ub_report_builder_t *b, int64_t ts);
void ub_report_add_field_i64(ub_report_builder_t *b, const char *name, int64_t value);
void ub_report_add_field_f64(ub_report_builder_t *b, const char *name, double value);
/* Escape hatch for compound sensors (e.g. NPK): `raw_json` must be a
 * complete, valid JSON value (typically an object) and is copied verbatim. */
void ub_report_add_field_raw(ub_report_builder_t *b, const char *name, const char *raw_json);
void ub_report_record_end(ub_report_builder_t *b);
void ub_report_add_ack(ub_report_builder_t *b, const char *cmd_id);
/* Returns total length written, or -1 if the buffer was too small at any
 * step (failure is sticky — check this return value, not each add_* call). */
int ub_report_end(ub_report_builder_t *b);

/* ---- response parsing -------------------------------------------------
 * Every parser expects `json` to be one complete top-level object. They
 * read "c" first; if it is non-zero the call still returns 0 (parsing
 * succeeded) and the caller should treat the other fields as absent and
 * read the "m" message with ub_parse_error_response if needed. Returns -1
 * only on a JSON shape/parse failure. */

typedef struct {
    int c;
    int64_t t;
    char n[UB_NONCE_MAX];
} ub_time_response_t;

int ub_parse_time_response(const char *json, ub_time_response_t *out);

typedef struct {
    int c;
    char token[UB_TOKEN_MAX];
    int64_t exp;
} ub_activate_response_t;

int ub_parse_activate_response(const char *json, ub_activate_response_t *out);

typedef struct {
    char id[UB_CMD_ID_MAX];
    char tp[UB_CMD_TP_MAX];
    const char *a_json; /* NULL if absent; otherwise points into `json` at the raw "a" object */
    size_t a_json_len;
} ub_cmd_t;

#define UB_MAX_CMDS 8

typedef struct {
    int c;
    int64_t t;
    int has_t;

    int has_cfg;
    int ci;
    int ui;
    /* fe (enabled sensor list) is intentionally not decoded here: it's a
     * variable-length string array and every caller so far only needs
     * ci/ui. Use ub_json_find_key(json, "fe") directly if you need it. */

    int cmd_count; /* may be less than what the server actually queued: capped at UB_MAX_CMDS */
    ub_cmd_t cmd[UB_MAX_CMDS];
} ub_report_response_t;

int ub_parse_report_response(const char *json, ub_report_response_t *out);

/* Generic {"c":N,"m":"..."} error envelope, used by every endpoint. */
int ub_parse_error_response(const char *json, int *code, char *msg, size_t msg_cap);

#ifdef __cplusplus
}
#endif

#endif /* UB_PROTOCOL_H */
