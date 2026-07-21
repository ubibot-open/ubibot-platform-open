/* Unit tests for the device-side protocol codec: request building, HMAC
 * signing (cross-checked against Go's crypto/hmac reference values), and
 * response parsing -- including the nak/prb/ota report-builder sections,
 * cfg.fe decoding, the config-poll response, and the set_probe/ota command
 * argument parsers added for the simulation build. No network involved. */
#include <stdio.h>
#include <string.h>

#include "ub_json.h"
#include "ub_protocol.h"
#include "ub_test.h"

#define SECRET "test-secret"
#define PID "ubibot_open_dev_v1"
#define SN "sn_ws1_20001_1"

static int approx(double a, double b) {
    double d = a - b;
    if (d < 0) d = -d;
    return d < 1e-9;
}

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

static void test_parse_report_response_full_with_fe(void) {
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
    UB_CHECK_EQ_INT(out.fe_count, 2);
    UB_CHECK_STREQ(out.fe[0], "temperature");
    UB_CHECK_STREQ(out.fe[1], "humidity");
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
    const char *json = "{\"c\":0,\"t\":1788950000}";
    ub_report_response_t out;
    UB_CHECK_EQ_INT(ub_parse_report_response(json, &out), 0);
    UB_CHECK(!out.has_cfg);
    UB_CHECK_EQ_INT(out.cmd_count, 0);
    UB_CHECK_EQ_INT(out.fe_count, 0);
}

static void test_parse_report_response_empty_cmd_array(void) {
    const char *json = "{\"c\":0,\"t\":1,\"cmd\":[]}";
    ub_report_response_t out;
    UB_CHECK_EQ_INT(ub_parse_report_response(json, &out), 0);
    UB_CHECK_EQ_INT(out.cmd_count, 0);
}

static void test_parse_poll_response(void) {
    const char *json =
        "{\"c\":0,\"cfg\":{\"ci\":10,\"ui\":60,\"fe\":[\"temperature\"]},"
        "\"cmd\":[{\"id\":\"c1\",\"tp\":\"reboot\"}]}";
    ub_poll_response_t out;
    UB_CHECK_EQ_INT(ub_parse_poll_response(json, &out), 0);
    UB_CHECK_EQ_INT(out.c, 0);
    UB_CHECK(out.has_cfg);
    UB_CHECK_EQ_INT(out.ci, 10);
    UB_CHECK_EQ_INT(out.ui, 60);
    UB_CHECK_EQ_INT(out.fe_count, 1);
    UB_CHECK_STREQ(out.fe[0], "temperature");
    UB_CHECK_EQ_INT(out.cmd_count, 1);
    UB_CHECK_STREQ(out.cmd[0].id, "c1");
    UB_CHECK_STREQ(out.cmd[0].tp, "reboot");
}

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
    char buf[8];
    ub_report_builder_t b;
    ub_report_begin(&b, buf, sizeof(buf), "some-long-device-id");
    ub_report_record_begin(&b, 1788950400);
    ub_report_add_field_f64(&b, "temperature", 25.6);
    ub_report_record_end(&b);
    int n = ub_report_end(&b);
    UB_CHECK_EQ_INT(n, -1);
}

static void test_report_builder_ack_nak_prb_ota(void) {
    char buf[512];
    ub_report_builder_t b;
    ub_report_begin(&b, buf, sizeof(buf), "d1");
    ub_report_record_begin(&b, 1);
    ub_report_add_field_i64(&b, "x", 1);
    ub_report_record_end(&b);
    ub_report_add_ack(&b, "c1");
    ub_report_add_nak(&b, "c2", 5, "bad args");
    ub_report_add_prb(&b, "probe1");
    ub_report_set_ota(&b, "c3", "1.2.3", "downloading", 42);
    int n = ub_report_end(&b);
    UB_CHECK(n > 0);

    const char *expect =
        "{\"did\":\"d1\",\"recs\":[{\"ts\":1,\"d\":{\"x\":1}}],"
        "\"ack\":[\"c1\"],"
        "\"nak\":[{\"id\":\"c2\",\"c\":5,\"m\":\"bad args\"}],"
        "\"prb\":[\"probe1\"],"
        "\"ota\":{\"id\":\"c3\",\"version\":\"1.2.3\",\"state\":\"downloading\",\"progress\":42}}";
    UB_CHECK_STREQ(buf, expect);
}

static void test_report_builder_nak_only_omits_ack_array(void) {
    char buf[256];
    ub_report_builder_t b;
    ub_report_begin(&b, buf, sizeof(buf), "d1");
    ub_report_record_begin(&b, 1);
    ub_report_add_field_i64(&b, "x", 1);
    ub_report_record_end(&b);
    ub_report_add_nak(&b, "c2", 5, "bad");
    int n = ub_report_end(&b);
    UB_CHECK(n > 0);
    UB_CHECK(strstr(buf, "\"ack\"") == NULL);
    UB_CHECK(strstr(buf, "\"nak\":[{\"id\":\"c2\",\"c\":5,\"m\":\"bad\"}]") != NULL);
}

static void test_parse_set_probe_args_upsert(void) {
    const char *a =
        "{\"op\":\"upsert\",\"pid\":\"p1\",\"key\":\"soil_temp\",\"iface\":\"rs485\","
        "\"proto\":\"modbus\",\"addr\":\"01\",\"fc\":3,\"reg\":100,\"cnt\":2,"
        "\"dtype\":\"int16\",\"byte_order\":\"be\",\"scale\":0.1,\"offset\":-40,"
        "\"ci\":60,\"timeout\":500,\"retry\":3}";
    ub_set_probe_args_t out;
    UB_CHECK_EQ_INT(ub_parse_set_probe_args(a, &out), 0);
    UB_CHECK_STREQ(out.op, "upsert");
    UB_CHECK_STREQ(out.pid, "p1");
    UB_CHECK(out.has_key);
    UB_CHECK_STREQ(out.key, "soil_temp");
    UB_CHECK(out.has_iface);
    UB_CHECK_STREQ(out.iface, "rs485");
    UB_CHECK(out.has_proto);
    UB_CHECK_STREQ(out.proto, "modbus");
    UB_CHECK(out.has_addr);
    UB_CHECK_STREQ(out.addr, "01");
    UB_CHECK(out.has_fc);
    UB_CHECK_EQ_INT(out.fc, 3);
    UB_CHECK(out.has_reg);
    UB_CHECK_EQ_INT(out.reg, 100);
    UB_CHECK(out.has_cnt);
    UB_CHECK_EQ_INT(out.cnt, 2);
    UB_CHECK(out.has_dtype);
    UB_CHECK_STREQ(out.dtype, "int16");
    UB_CHECK(out.has_byte_order);
    UB_CHECK_STREQ(out.byte_order, "be");
    UB_CHECK(out.has_scale);
    UB_CHECK(approx(out.scale, 0.1));
    UB_CHECK(out.has_offset);
    UB_CHECK(approx(out.offset, -40.0));
    UB_CHECK(out.has_ci);
    UB_CHECK_EQ_INT(out.ci, 60);
    UB_CHECK(out.has_timeout);
    UB_CHECK_EQ_INT(out.timeout, 500);
    UB_CHECK(out.has_retry);
    UB_CHECK_EQ_INT(out.retry, 3);
}

static void test_parse_set_probe_args_remove(void) {
    const char *a = "{\"op\":\"remove\",\"pid\":\"p1\"}";
    ub_set_probe_args_t out;
    UB_CHECK_EQ_INT(ub_parse_set_probe_args(a, &out), 0);
    UB_CHECK_STREQ(out.op, "remove");
    UB_CHECK_STREQ(out.pid, "p1");
    UB_CHECK(!out.has_key);
    UB_CHECK(!out.has_scale);
}

static void test_parse_ota_args_start(void) {
    const char *a =
        "{\"action\":\"start\",\"version\":\"1.2.3\",\"url\":\"/api/v1/ota/firmware/abc\","
        "\"size\":123456,\"sha256\":\"deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef\","
        "\"sig\":\"sigdata\",\"force\":true}";
    ub_ota_args_t out;
    UB_CHECK_EQ_INT(ub_parse_ota_args(a, &out), 0);
    UB_CHECK_STREQ(out.action, "start");
    UB_CHECK_STREQ(out.version, "1.2.3");
    UB_CHECK_STREQ(out.url, "/api/v1/ota/firmware/abc");
    UB_CHECK(out.has_size);
    UB_CHECK_EQ_INT((long)out.size, 123456);
    UB_CHECK_STREQ(out.sha256_hex,
                   "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef");
    UB_CHECK(out.has_sig);
    UB_CHECK_STREQ(out.sig, "sigdata");
    UB_CHECK(out.has_force);
    UB_CHECK(out.force);
}

static void test_parse_ota_args_cancel(void) {
    const char *a = "{\"action\":\"cancel\"}";
    ub_ota_args_t out;
    UB_CHECK_EQ_INT(ub_parse_ota_args(a, &out), 0);
    UB_CHECK_STREQ(out.action, "cancel");
    UB_CHECK(!out.has_size);
    UB_CHECK(!out.has_sig);
    UB_CHECK(!out.has_force);
}

int main(void) {
    test_build_time_request();
    test_build_time_request_buffer_too_small();
    test_build_activate_request_with_nonce();
    test_build_activate_request_local_clock();
    test_parse_time_response();
    test_parse_activate_response();
    test_parse_error_response();
    test_parse_report_response_full_with_fe();
    test_parse_report_response_minimal();
    test_parse_report_response_empty_cmd_array();
    test_parse_poll_response();
    test_json_find_key_not_confused_by_nesting();
    test_json_get_double();
    test_report_builder_basic();
    test_report_builder_no_ack();
    test_report_builder_overflow();
    test_report_builder_ack_nak_prb_ota();
    test_report_builder_nak_only_omits_ack_array();
    test_parse_set_probe_args_upsert();
    test_parse_set_probe_args_remove();
    test_parse_ota_args_start();
    test_parse_ota_args_cancel();
    UB_TEST_SUMMARY();
}
