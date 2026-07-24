/* Unit tests for the device-side protocol codec: request building and
 * response parsing for the two-endpoint protocol (docs/UbiBot开放平台硬件
 * 通信协议.md). No network involved, no crypto involved -- the protocol has
 * neither any more. Expected JSON strings are transcribed character-for-
 * character from the doc's own examples wherever the doc gives one. */
#include <stdio.h>
#include <string.h>

#include "ub_json.h"
#include "ub_protocol.h"
#include "ub_test.h"

#define PID "ubibot_open_dev_v1"
#define SN "sn_ws1_20001_1"

static int approx(double a, double b) {
    double d = a - b;
    if (d < 0) d = -d;
    return d < 1e-9;
}

/* ---- /auth/time ---------------------------------------------------------*/

static void test_build_time_request(void) {
    ub_identity_t id = {PID, SN};
    char buf[128];
    int n = ub_build_time_request(&id, buf, sizeof(buf));
    UB_CHECK(n > 0);
    UB_CHECK_STREQ(buf, "{\"pid\":\"" PID "\",\"sn\":\"" SN "\"}");
}

static void test_build_time_request_buffer_too_small(void) {
    ub_identity_t id = {PID, SN};
    char buf[8];
    int n = ub_build_time_request(&id, buf, sizeof(buf));
    UB_CHECK_EQ_INT(n, -1);
}

static void test_parse_time_response(void) {
    const char *json = "{\"c\":0,\"t\":1788950400}";
    ub_time_response_t out;
    UB_CHECK_EQ_INT(ub_parse_time_response(json, &out), 0);
    UB_CHECK_EQ_INT(out.c, 0);
    UB_CHECK_EQ_INT(out.t, 1788950400);
}

static void test_parse_time_response_error(void) {
    const char *json = "{\"c\":1003,\"m\":\"bad body\"}";
    ub_time_response_t out;
    UB_CHECK_EQ_INT(ub_parse_time_response(json, &out), 0);
    UB_CHECK_EQ_INT(out.c, UB_CODE_MALFORMED_BODY);
}

/* ---- /data/report --------------------------------------------------------*/

static void test_report_builder_matches_doc_example(void) {
    /* Transcribed from protocol §4's request example. */
    char buf[512];
    ub_report_builder_t b;
    ub_report_begin(&b, buf, sizeof(buf), PID, SN, 1788950400);

    ub_report_payload_begin(&b, 1788950400);
    ub_report_add_field(&b, 1, 25.6);
    ub_report_add_field(&b, 2, 60.2);
    ub_report_payload_end(&b);

    ub_report_payload_begin(&b, 1788951000);
    ub_report_add_field(&b, 1, 25.8);
    ub_report_add_field(&b, 2, 59.9);
    ub_report_add_field(&b, 3, 812);
    ub_report_payload_end(&b);

    int n = ub_report_end(&b);
    UB_CHECK(n > 0);

    const char *expect =
        "{\"pid\":\"" PID "\",\"sn\":\"" SN "\",\"ts\":1788950400,\"payloads\":["
        "{\"ts\":1788950400,\"feed\":{\"field1\":25.6,\"field2\":60.2}},"
        "{\"ts\":1788951000,\"feed\":{\"field1\":25.8,\"field2\":59.9,\"field3\":812}}"
        "]}";
    UB_CHECK_STREQ(buf, expect);
}

static void test_report_builder_single_payload_single_field(void) {
    /* A device may report only field1 -- the doc explicitly allows omitting
     * everything else, and starting from a field other than field1. */
    char buf[256];
    ub_report_builder_t b;
    ub_report_begin(&b, buf, sizeof(buf), "p1", "s1", 100);
    ub_report_payload_begin(&b, 100);
    ub_report_add_field(&b, 4, 3.3);
    ub_report_payload_end(&b);
    int n = ub_report_end(&b);
    UB_CHECK(n > 0);
    UB_CHECK_STREQ(buf, "{\"pid\":\"p1\",\"sn\":\"s1\",\"ts\":100,\"payloads\":[{\"ts\":100,\"feed\":{\"field4\":3.3}}]}");
}

static void test_report_builder_no_payloads(void) {
    char buf[128];
    ub_report_builder_t b;
    ub_report_begin(&b, buf, sizeof(buf), "p1", "s1", 100);
    int n = ub_report_end(&b);
    UB_CHECK(n > 0);
    UB_CHECK_STREQ(buf, "{\"pid\":\"p1\",\"sn\":\"s1\",\"ts\":100,\"payloads\":[]}");
}

static void test_report_builder_overflow(void) {
    char buf[8];
    ub_report_builder_t b;
    ub_report_begin(&b, buf, sizeof(buf), "some-long-pid", "some-long-sn", 1788950400);
    ub_report_payload_begin(&b, 1788950400);
    ub_report_add_field(&b, 1, 25.6);
    ub_report_payload_end(&b);
    int n = ub_report_end(&b);
    UB_CHECK_EQ_INT(n, -1);
}

static void test_report_builder_field_out_of_range(void) {
    char buf[256];
    ub_report_builder_t b;
    ub_report_begin(&b, buf, sizeof(buf), "p1", "s1", 100);
    ub_report_payload_begin(&b, 100);
    ub_report_add_field(&b, 21, 1.0); /* only field1..field20 are valid */
    ub_report_payload_end(&b);
    int n = ub_report_end(&b);
    UB_CHECK_EQ_INT(n, -1);

    char buf2[256];
    ub_report_builder_t b2;
    ub_report_begin(&b2, buf2, sizeof(buf2), "p1", "s1", 100);
    ub_report_payload_begin(&b2, 100);
    ub_report_add_field(&b2, 0, 1.0);
    ub_report_payload_end(&b2);
    UB_CHECK_EQ_INT(ub_report_end(&b2), -1);
}

static void test_parse_report_response_ok_with_t(void) {
    const char *json = "{\"c\":0,\"t\":1788950400}";
    ub_report_response_t out;
    UB_CHECK_EQ_INT(ub_parse_report_response(json, &out), 0);
    UB_CHECK_EQ_INT(out.c, UB_CODE_OK);
    UB_CHECK(out.has_t);
    UB_CHECK_EQ_INT(out.t, 1788950400);
}

static void test_parse_report_response_error_has_no_t_requirement(void) {
    const char *json = "{\"c\":1103,\"m\":\"device disabled\"}";
    ub_report_response_t out;
    UB_CHECK_EQ_INT(ub_parse_report_response(json, &out), 0);
    UB_CHECK_EQ_INT(out.c, UB_CODE_DEVICE_DISABLED);
    UB_CHECK(!out.has_t);
}

static void test_parse_error_response(void) {
    const char *json = "{\"c\":1002,\"m\":\"timestamp out of window\"}";
    int code;
    char msg[64];
    UB_CHECK_EQ_INT(ub_parse_error_response(json, &code, msg, sizeof(msg)), 0);
    UB_CHECK_EQ_INT(code, UB_CODE_TIMESTAMP_OUT_OF_RANGE);
    UB_CHECK_STREQ(msg, "timestamp out of window");
}

static void test_all_error_codes_from_doc(void) {
    UB_CHECK_EQ_INT(UB_CODE_OK, 0);
    UB_CHECK_EQ_INT(UB_CODE_TIMESTAMP_OUT_OF_RANGE, 1002);
    UB_CHECK_EQ_INT(UB_CODE_MALFORMED_BODY, 1003);
    UB_CHECK_EQ_INT(UB_CODE_DEVICE_DISABLED, 1103);
    UB_CHECK_EQ_INT(UB_CODE_RATE_LIMITED, 1900);
    UB_CHECK_EQ_INT(UB_CODE_SERVER_ERROR, 5000);
}

/* ---- ub_json.h (generic parser, unchanged, still worth a smoke test) ----*/

static void test_json_find_key_not_confused_by_nesting(void) {
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

static void test_json_get_double(void) {
    const char *json = "{\"scale\":0.1,\"offset\":-40,\"n\":42}";
    double v;
    UB_CHECK_EQ_INT(ub_json_get_double(ub_json_find_key(json, "scale"), &v), 0);
    UB_CHECK(approx(v, 0.1));
    UB_CHECK_EQ_INT(ub_json_get_double(ub_json_find_key(json, "offset"), &v), 0);
    UB_CHECK(approx(v, -40.0));
    UB_CHECK_EQ_INT(ub_json_get_double(ub_json_find_key(json, "n"), &v), 0);
    UB_CHECK(approx(v, 42.0));
}

int main(void) {
    test_build_time_request();
    test_build_time_request_buffer_too_small();
    test_parse_time_response();
    test_parse_time_response_error();
    test_report_builder_matches_doc_example();
    test_report_builder_single_payload_single_field();
    test_report_builder_no_payloads();
    test_report_builder_overflow();
    test_report_builder_field_out_of_range();
    test_parse_report_response_ok_with_t();
    test_parse_report_response_error_has_no_t_requirement();
    test_parse_error_response();
    test_all_error_codes_from_doc();
    test_json_find_key_not_confused_by_nesting();
    test_json_get_double();
    UB_TEST_SUMMARY();
}
