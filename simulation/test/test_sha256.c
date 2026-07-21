/* SHA-256 and HMAC-SHA256 vs. known-answer test vectors. The SHA-256 and
 * RFC 4231 HMAC vectors are cross-checked against Go's standard library
 * (crypto/sha256, crypto/hmac) rather than transcribed from memory. */
#include <string.h>

#include "ub_hmac_sha256.h"
#include "ub_sha256.h"
#include "ub_test.h"

static void check_digest(const char *label, const uint8_t *digest, const char *expected_hex) {
    char hex[65];
    ub_hex_encode(digest, 32, hex);
    ub_test_count++;
    if (strcmp(hex, expected_hex) != 0) {
        ub_test_failures++;
        fprintf(stderr, "FAIL %s: got %s want %s\n", label, hex, expected_hex);
    }
}

static void test_sha256_vectors(void) {
    uint8_t digest[32];

    ub_sha256((const uint8_t *)"", 0, digest);
    check_digest("sha256(\"\")", digest,
                 "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855");

    ub_sha256((const uint8_t *)"abc", 3, digest);
    check_digest("sha256(\"abc\")", digest,
                 "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad");
}

static void test_sha256_streaming_matches_oneshot(void) {
    /* Feeding "abc" one byte at a time must match the one-shot result —
     * exercises the block-boundary logic in ub_sha256_update/final. */
    uint8_t oneshot[32], streamed[32];
    ub_sha256((const uint8_t *)"abc", 3, oneshot);

    ub_sha256_ctx_t ctx;
    ub_sha256_init(&ctx);
    ub_sha256_update(&ctx, (const uint8_t *)"a", 1);
    ub_sha256_update(&ctx, (const uint8_t *)"b", 1);
    ub_sha256_update(&ctx, (const uint8_t *)"c", 1);
    ub_sha256_final(&ctx, streamed);

    UB_CHECK(memcmp(oneshot, streamed, 32) == 0);
}

static void test_hmac_rfc4231_case1(void) {
    uint8_t key[20];
    memset(key, 0x0b, sizeof(key));
    uint8_t digest[32];
    ub_hmac_sha256(key, sizeof(key), (const uint8_t *)"Hi There", 8, digest);
    check_digest("hmac RFC4231#1", digest,
                 "b0344c61d8db38535ca8afceaf0bf12b881dc200c9833da726e9376c2e32cff7");
}

static void test_hmac_rfc4231_case2(void) {
    uint8_t digest[32];
    ub_hmac_sha256((const uint8_t *)"Jefe", 4,
                   (const uint8_t *)"what do ya want for nothing?", 28, digest);
    check_digest("hmac RFC4231#2", digest,
                 "5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843");
}

static void test_hmac_streaming_matches_oneshot(void) {
    uint8_t oneshot[32], streamed[32];
    ub_hmac_sha256((const uint8_t *)"test-secret", 11, (const uint8_t *)"hello world", 11,
                   oneshot);

    ub_hmac_sha256_ctx_t ctx;
    ub_hmac_sha256_init(&ctx, (const uint8_t *)"test-secret", 11);
    ub_hmac_sha256_update(&ctx, (const uint8_t *)"hello", 5);
    ub_hmac_sha256_update(&ctx, (const uint8_t *)" world", 6);
    ub_hmac_sha256_final(&ctx, streamed);

    UB_CHECK(memcmp(oneshot, streamed, 32) == 0);
}

static void test_hex_encode(void) {
    uint8_t data[3] = {0x00, 0xab, 0xff};
    char out[7];
    ub_hex_encode(data, 3, out);
    UB_CHECK_STREQ(out, "00abff");
}

int main(void) {
    test_sha256_vectors();
    test_sha256_streaming_matches_oneshot();
    test_hmac_rfc4231_case1();
    test_hmac_rfc4231_case2();
    test_hmac_streaming_matches_oneshot();
    test_hex_encode();
    UB_TEST_SUMMARY();
}
