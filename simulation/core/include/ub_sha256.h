/* Compact, dependency-free SHA-256 (FIPS 180-4). Streaming API only, no
 * heap allocation, so a caller on a memory-constrained MCU can put the
 * context on the stack (~110 bytes) and feed data incrementally.
 *
 * Portability: pure C11, only <stddef.h>/<stdint.h>. No OS, no libc beyond
 * memcpy/memset (pulled in by the .c file) — safe to compile unmodified
 * against any embedded libc (newlib, picolibc, etc.) for a real FreeRTOS
 * target. */
#ifndef UB_SHA256_H
#define UB_SHA256_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define UB_SHA256_DIGEST_LEN 32
#define UB_SHA256_BLOCK_LEN 64

typedef struct {
    uint32_t state[8];
    uint64_t total_len;
    uint8_t buf[UB_SHA256_BLOCK_LEN];
    size_t buf_len;
} ub_sha256_ctx_t;

void ub_sha256_init(ub_sha256_ctx_t *ctx);
void ub_sha256_update(ub_sha256_ctx_t *ctx, const uint8_t *data, size_t len);
void ub_sha256_final(ub_sha256_ctx_t *ctx, uint8_t out[UB_SHA256_DIGEST_LEN]);

/* One-shot convenience wrapper. */
void ub_sha256(const uint8_t *data, size_t len, uint8_t out[UB_SHA256_DIGEST_LEN]);

#ifdef __cplusplus
}
#endif

#endif /* UB_SHA256_H */
