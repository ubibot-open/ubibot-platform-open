/* HMAC-SHA256 (RFC 2104 / FIPS 198-1), built on ub_sha256's streaming API.
 * The incremental init/update/final form lets a caller sign a message that
 * is naturally split into several pieces (e.g. pid + sn + ts + n) without
 * concatenating them into one buffer first — useful on memory-constrained
 * targets. */
#ifndef UB_HMAC_SHA256_H
#define UB_HMAC_SHA256_H

#include <stddef.h>
#include <stdint.h>

#include "ub_sha256.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef struct {
    ub_sha256_ctx_t inner;
    uint8_t opad_key[UB_SHA256_BLOCK_LEN];
} ub_hmac_sha256_ctx_t;

void ub_hmac_sha256_init(ub_hmac_sha256_ctx_t *ctx, const uint8_t *key, size_t key_len);
void ub_hmac_sha256_update(ub_hmac_sha256_ctx_t *ctx, const uint8_t *data, size_t len);
void ub_hmac_sha256_final(ub_hmac_sha256_ctx_t *ctx, uint8_t out[UB_SHA256_DIGEST_LEN]);

/* One-shot convenience wrapper for a single contiguous message. */
void ub_hmac_sha256(const uint8_t *key, size_t key_len, const uint8_t *msg, size_t msg_len,
                     uint8_t out[UB_SHA256_DIGEST_LEN]);

/* Lowercase hex-encodes `len` bytes into `out`, which must be at least
 * len*2+1 bytes (the +1 is the NUL terminator). */
void ub_hex_encode(const uint8_t *data, size_t len, char *out);

#ifdef __cplusplus
}
#endif

#endif /* UB_HMAC_SHA256_H */
