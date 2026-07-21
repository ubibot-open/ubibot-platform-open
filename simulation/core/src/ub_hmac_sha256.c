#include "ub_hmac_sha256.h"

#include <string.h>

void ub_hmac_sha256_init(ub_hmac_sha256_ctx_t *ctx, const uint8_t *key, size_t key_len) {
    uint8_t key_block[UB_SHA256_BLOCK_LEN];
    memset(key_block, 0, sizeof(key_block));

    if (key_len > UB_SHA256_BLOCK_LEN) {
        ub_sha256(key, key_len, key_block); /* fills first 32 bytes, rest stays 0 */
    } else {
        memcpy(key_block, key, key_len);
    }

    uint8_t ipad_key[UB_SHA256_BLOCK_LEN];
    for (size_t i = 0; i < UB_SHA256_BLOCK_LEN; i++) {
        ipad_key[i] = key_block[i] ^ 0x36;
        ctx->opad_key[i] = key_block[i] ^ 0x5c;
    }

    ub_sha256_init(&ctx->inner);
    ub_sha256_update(&ctx->inner, ipad_key, UB_SHA256_BLOCK_LEN);
}

void ub_hmac_sha256_update(ub_hmac_sha256_ctx_t *ctx, const uint8_t *data, size_t len) {
    ub_sha256_update(&ctx->inner, data, len);
}

void ub_hmac_sha256_final(ub_hmac_sha256_ctx_t *ctx, uint8_t out[UB_SHA256_DIGEST_LEN]) {
    uint8_t inner_digest[UB_SHA256_DIGEST_LEN];
    ub_sha256_final(&ctx->inner, inner_digest);

    ub_sha256_ctx_t outer;
    ub_sha256_init(&outer);
    ub_sha256_update(&outer, ctx->opad_key, UB_SHA256_BLOCK_LEN);
    ub_sha256_update(&outer, inner_digest, UB_SHA256_DIGEST_LEN);
    ub_sha256_final(&outer, out);
}

void ub_hmac_sha256(const uint8_t *key, size_t key_len, const uint8_t *msg, size_t msg_len,
                     uint8_t out[UB_SHA256_DIGEST_LEN]) {
    ub_hmac_sha256_ctx_t ctx;
    ub_hmac_sha256_init(&ctx, key, key_len);
    ub_hmac_sha256_update(&ctx, msg, msg_len);
    ub_hmac_sha256_final(&ctx, out);
}

void ub_hex_encode(const uint8_t *data, size_t len, char *out) {
    static const char digits[] = "0123456789abcdef";
    for (size_t i = 0; i < len; i++) {
        out[i * 2] = digits[data[i] >> 4];
        out[i * 2 + 1] = digits[data[i] & 0x0f];
    }
    out[len * 2] = '\0';
}
