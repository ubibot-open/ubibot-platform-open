/* Unit tests for the device-side protocol codec: request building, HMAC
 * signing (cross-checked against Go's crypto/hmac reference values -- see
 * scratchpad/gen_vectors.go), and response parsing. No network involved. */
#include <stdio.h>
#include <string.h>

#include "ub_json.h"
#include "ub_protocol.h"
#include "ub_test.h"

#define SECRET "test-secret"
#define PID "ubibot_open_dev_v1"
#define SN "sn_ws1_20001_1"

static void test_build_time_request(void) {
    ub_identity_t id = {PID, SN, SECRET};
    char buf[256];
    int n = ub_build_time_request(&id, buf, sizeof(buf));
    UB_CHECK(n > 0);
    UB_CHECK_STREQ(buf,
        "{\"pid\":\"" PID "\",\"sn\":\"" SN "\",\"sign\":\"f30f94e47d8f2aba36f58d198694b9d815cbf197de89779162114c78e86502f6\"}");
}

static void test_build_time_request_buffer_too_small(void) {
    ub_identity_t id = {PID, SN, SECRET};
    char buf[8];
    int n = ub_build_time_request(&id, buf, sizeof(buf));
    UB_CHECK_EQ_INT(n, -1);
}

static void test_build_activate_request_with_nonce(void) {
    ub_identity_t id = {PID, SN, SECRET};
    char buf[320];
    int n = ub_build_activate_request(&id, 1788950400, "7f3a9c21", buf, sizeof(buf));
    UB_CHECK(n > 0);
    UB_CHECK_STREQ(buf,
        "{\"pid\":\"" PID "\",\"sn\":\"" SN "\",\"ts\":1788950400,\"n\":\"7f3a9c21\","
        "\"sign\":\"da1dfb1be4f6f07b43909d3559d16ec9a7d94e0e4e78df4bff10406312823efa\"}");
}

static void test_build_activate_request_local_clock(void) {
    ub_identity_t id = {PID, SN, SECRET};
    char buf[320];
    int n = ub_build_activate_request(&id, 1788950400, NULL, buf, sizeof(buf));
    UB_CHECK(n > 0);
    UB_CHECK_STREQ(buf,
        "{\"pid\":\"" PID "\",\"sn\":\"" SN "\",\"ts\":1788950400,"
        "\"sign\":\"c5eabdf74e99807ff8111acb750840bfe85f4aa127cfa88141a99fa9e271c5d0\"}");
    /* Must not contain an "n" field at all when there's no nonce. */
    UB_CHECK(strstr(buf, "\"n\":") == NULL);
}

static void test_parse_time_response(void) {
    const char *json = "{\"c\":0,\"t\":1788950400,\"n\":\"7f3a9c21\"}";
    ub_time_response_t out;
    UB_CHECK_EQ_INT(ub_parse_time_response(json, &out), 0);
    UB_CHECK_EQ_INT(out.c, 0);
    UB_CHECK_EQ_INT(out.t, 1788950400);
    UB_CHECK_STREQ(out.n, "7f3a9c21");
}

static void test_parse_activate_response(void) {
    const char *json = "{\"c\":0,\"token\":\"abc123def456\",\"exp\":86400}";
    ub_activate_response_t out;
    UB_CHECK_EQ_INT(ub_parse_activate_response(json, &out), 0);
    UB_CHECK_EQ_INT(out.c, 0);
    UB_CHECK_STREQ(out.token, "abc123def456");
    UB_CHECK_EQ_INT(out.exp, 86400);
}

static void test_parse_error_response(void) {
    const char *json = "{\"c\":1002,\"m\":\"sign mismatch\"}";
    int code;
    char msg[64];
    UB_CHECK_EQ_INT(ub_parse_error_response(json, &code, msg, sizeof(msg)), 0);
    UB_CHECK_EQ_INT(code, UB_CODE_SIGN_MISMATCH);
    UB_CHECK_STREQ(msg, "sign mismatch");
}

static void test_parse_report_response_full(void) {
    const char *json =
        "{\"c\":0,\"t\":1788950000,"
        "\"cfg\":{\"ci\":30,\"ui\":600,\"fe\":[\"temperature\",\"humidity\"]},"
        "\"cmd\":["
        "{\"id\":\"c1009\",\"tp\":\"set_cfg\",\"a\":{\"ui\":300}},"
        "{\"id\":\"c1010\",\"tp\":\"reboot\"}"
        "]}";
    ub_report_response_t out;
    UB_CHECK_EQ_INT(ub_parse_report_response(json, &out), 0);
    UB_CHECK_EQ_INT(out.c, 0);
    UB_CHECK(out.has_t);
    UB_CHECK_EQ_INT(out.t, 1788950000);
    UB_CHECK(out.has_cfg);
    UB_CHECK_EQ_INT(out.ci, 30);
    UB_CHECK_EQ_INT(out.ui, 600);
    UB_CHECK_EQ_INT(out.cmd_count, 2);
    UB_CHECK_STREQ(out.cmd[0].id, "c1009");
    UB_CHECK_STREQ(out.cmd[0].tp, "set_cfg");
    UB_CHECK(out.cmd[0].a_json != NULL);
    UB_CHECK_EQ_INT((long)out.cmd[0].a_json_len, (long)strlen("{\"ui\":300}"));
    UB_CHECK(strncmp(out.cmd[0].a_json, "{\"ui\":300}", strlen("{\"ui\":300}")) == 0);
    UB_CHECK_STREQ(out.cmd[1].id, "c1010");
    UB_CHECK_STREQ(out.cmd[1].tp, "reboot");
    UB_CHECK(out.cmd[1].a_json == NULL);
}

static void test_parse_report_response_minimal(void) {
    /* No cfg, no cmd -- the common case, kept small on purpose. */
    const char *json = "{\"c\":0,\"t\":1788950000}";
    ub_report_response_t out;
    UB_CHECK_EQ_INT(ub_parse_report_response(json, &out), 0);
    UB_CHECK(!out.has_cfg);
    UB_CHECK_EQ_INT(out.cmd_count, 0);
}

static void test_parse_report_response_empty_cmd_array(void) {
    const char *json = "{\"c\":0,\"t\":1,\"cmd\":[]}";
    ub_report_response_t out;
    UB_CHECK_EQ_INT(ub_parse_report_response(json, &out), 0);
    UB_CHECK_EQ_INT(out.cmd_count, 0);
}

static void test_json_find_key_not_confused_by_nesting(void) {
    /* A naive substring search for "\"a\":" would match the nested "a"
     * inside the "a" object's own value; ub_json_find_key must not. */
    const char *json = "{\"a\":{\"a\":1},\"b\":2}";
    const char *v = ub_json_find_key(json, "b");
    int64_t b;
    UB_CHECK(v != NULL);
    UB_CHECK_EQ_INT(ub_json_get_int64(v, &b), 0);
    UB_CHECK_EQ_INT(b, 2);

    const char *a = ub_json_find_key(json, "a");
    UB_CHECK(a != NULL);
    UB_CHECK(ub_json_is_object(a));
    int64_t inner;
    const char *inner_v = ub_json_find_key(a, "a");
    UB_CHECK(inner_v != NULL);
    UB_CHECK_EQ_INT(ub_json_get_int64(inner_v, &inner), 0);
    UB_CHECK_EQ_INT(inner, 1);
}

static void test_report_builder_basic(void) {
    char buf[512];
    ub_report_builder_t b;
    ub_report_begin(&b, buf, sizeof(buf), "ws1-20001-1");

    ub_report_record_begin(&b, 1788950400);
    ub_report_add_field_f64(&b, "temperature", 25.6);
    ub_report_add_field_f64(&b, "humidity", 60.2);
    ub_report_record_end(&b);

    ub_report_record_begin(&b, 1788951000);
    ub_report_add_field_f64(&b, "temperature", 25.8);
    ub_report_add_field_raw(&b, "npk", "{\"n\":120,\"p\":100,\"k\":100}");
    ub_report_record_end(&b);

    ub_report_add_ack(&b, "c1007");
    ub_report_add_ack(&b, "c1008");

    int n = ub_report_end(&b);
    UB_CHECK(n > 0);

    const char *expect =
        "{\"did\":\"ws1-20001-1\",\"recs\":["
        "{\"ts\":1788950400,\"d\":{\"temperature\":25.6,\"humidity\":60.2}},"
        "{\"ts\":1788951000,\"d\":{\"temperature\":25.8,\"npk\":{\"n\":120,\"p\":100,\"k\":100}}}"
        "],\"ack\":[\"c1007\",\"c1008\"]}";
    UB_CHECK_STREQ(buf, expect);

    /* And it must round-trip through our own parser's building blocks. */
    const char *did_v = ub_json_find_key(buf, "did");
    char did[32];
    UB_CHECK_EQ_INT(ub_json_get_string(did_v, did, sizeof(did)), 0);
    UB_CHECK_STREQ(did, "ws1-20001-1");
}

static void test_report_builder_no_ack(void) {
    char buf[256];
    ub_report_builder_t b;
    ub_report_begin(&b, buf, sizeof(buf), "d1");
    ub_report_record_begin(&b, 1);
    ub_report_add_field_i64(&b, "x", 42);
    ub_report_record_end(&b);
    int n = ub_report_end(&b);
    UB_CHECK(n > 0);
    UB_CHECK(strstr(buf, "\"ack\"") == NULL);
}

static void test_report_builder_overflow(void) {
    char buf[8]; /* far too small */
    ub_report_builder_t b;
    ub_report_begin(&b, buf, sizeof(buf), "some-long-device-id");
    ub_report_record_begin(&b, 1788950400);
    ub_report_add_field_f64(&b, "temperature", 25.6);
    ub_report_record_end(&b);
    int n = ub_report_end(&b);
    UB_CHECK_EQ_INT(n, -1);
}

int main(void) {
    test_build_time_request();
    test_build_time_request_buffer_too_small();
    test_build_activate_request_with_nonce();
    test_build_activate_request_local_clock();
    test_parse_time_response();
    test_parse_activate_response();
    test_parse_error_response();
    test_parse_report_response_full();
    test_parse_report_response_minimal();
    test_parse_report_response_empty_cmd_array();
    test_json_find_key_not_confused_by_nesting();
    test_report_builder_basic();
    test_report_builder_no_ack();
    test_report_builder_overflow();
    UB_TEST_SUMMARY();
}
