/* Integration test: drives the C client codec over a real TCP connection
 * against a *running* instance of cmd/server. Exercises the full device
 * lifecycle: time sync -> activate (nonce path) -> report -> re-activate
 * (local-clock path) -> a couple of error paths -- the things a real
 * device does that a pure unit test can't reach without a socket.
 *
 * Usage: test_integration [host] [port]
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "ub_protocol.h"
#include "ub_test.h"
#include "ub_transport_sockets.h"

/* Must match the demo device seeded in cmd/server/main.go. */
#define PID "ubibot_open_dev_v1"
#define SN "sn_ws1_20001_1"
#define SECRET "demo-secret-change-me"

static ub_transport_t g_tr;
static const char *g_host;
static int g_port;

static int do_post(const char *path, const char *body, const char *token, int *status,
                    char *resp, size_t resp_cap, size_t *resp_len) {
    return g_tr.post(g_tr.user_ctx, g_host, g_port, path, body, strlen(body), token, status, resp,
                      resp_cap, resp_len);
}

static void test_full_lifecycle(void) {
    ub_identity_t id = {PID, SN, SECRET};
    char req[320];
    char resp[2048];
    int status;
    size_t resp_len;

    /* 1. Time sync: no clock needed yet. */
    int n = ub_build_time_request(&id, req, sizeof(req));
    UB_CHECK(n > 0);
    UB_CHECK_EQ_INT(do_post("/api/v1/auth/time", req, NULL, &status, resp, sizeof(resp), &resp_len),
                    0);
    UB_CHECK_EQ_INT(status, 200);

    ub_time_response_t ts_resp;
    UB_CHECK_EQ_INT(ub_parse_time_response(resp, &ts_resp), 0);
    UB_CHECK_EQ_INT(ts_resp.c, UB_CODE_OK);
    UB_CHECK(ts_resp.n[0] != '\0');
    int64_t server_time = ts_resp.t;

    /* 2. Activate using the nonce from step 1 -- the clockless path. */
    n = ub_build_activate_request(&id, server_time, ts_resp.n, req, sizeof(req));
    UB_CHECK(n > 0);
    UB_CHECK_EQ_INT(
        do_post("/api/v1/auth/activate", req, NULL, &status, resp, sizeof(resp), &resp_len), 0);
    UB_CHECK_EQ_INT(status, 200);

    ub_activate_response_t act_resp;
    UB_CHECK_EQ_INT(ub_parse_activate_response(resp, &act_resp), 0);
    UB_CHECK_EQ_INT(act_resp.c, UB_CODE_OK);
    UB_CHECK(act_resp.token[0] != '\0');
    UB_CHECK(act_resp.exp > 0);
    char token[UB_TOKEN_MAX];
    strncpy(token, act_resp.token, sizeof(token) - 1);
    token[sizeof(token) - 1] = '\0';

    /* Reusing the same nonce must now fail: it was consumed by step 2. */
    UB_CHECK_EQ_INT(
        do_post("/api/v1/auth/activate", req, NULL, &status, resp, sizeof(resp), &resp_len), 0);
    UB_CHECK_EQ_INT(status, 400);
    int err_code;
    char err_msg[128];
    UB_CHECK_EQ_INT(ub_parse_error_response(resp, &err_code, err_msg, sizeof(err_msg)), 0);
    UB_CHECK_EQ_INT(err_code, UB_CODE_SIGN_MISMATCH);

    /* 3. Re-activate on the local-clock path (no nonce), using the
     * server time we already learned -- should still be well inside the
     * +-5 minute window given how fast this test runs. */
    n = ub_build_activate_request(&id, server_time, NULL, req, sizeof(req));
    UB_CHECK(n > 0);
    UB_CHECK_EQ_INT(
        do_post("/api/v1/auth/activate", req, NULL, &status, resp, sizeof(resp), &resp_len), 0);
    UB_CHECK_EQ_INT(status, 200);
    UB_CHECK_EQ_INT(ub_parse_activate_response(resp, &act_resp), 0);
    UB_CHECK_EQ_INT(act_resp.c, UB_CODE_OK);

    /* 4. Report two batched records using the token from step 2. */
    ub_report_builder_t b;
    ub_report_begin(&b, req, sizeof(req), SN);
    ub_report_record_begin(&b, server_time);
    ub_report_add_field_f64(&b, "temperature", 25.6);
    ub_report_add_field_f64(&b, "humidity", 60.2);
    ub_report_record_end(&b);
    ub_report_record_begin(&b, server_time + 600);
    ub_report_add_field_f64(&b, "temperature", 25.8);
    ub_report_add_field_raw(&b, "npk", "{\"n\":120,\"p\":100,\"k\":100}");
    ub_report_record_end(&b);
    n = ub_report_end(&b);
    UB_CHECK(n > 0);

    UB_CHECK_EQ_INT(
        do_post("/api/v1/data/report", req, token, &status, resp, sizeof(resp), &resp_len), 0);
    UB_CHECK_EQ_INT(status, 200);

    ub_report_response_t rep_resp;
    UB_CHECK_EQ_INT(ub_parse_report_response(resp, &rep_resp), 0);
    UB_CHECK_EQ_INT(rep_resp.c, UB_CODE_OK);

    /* Response must actually be small -- the whole point of the protocol's
     * field-name choices and "send cfg/cmd only when present" rule. */
    UB_CHECK(resp_len < 1024);
}

static void test_report_rejects_bad_token(void) {
    char req[256], resp[512];
    int status;
    size_t resp_len;

    ub_report_builder_t b;
    ub_report_begin(&b, req, sizeof(req), SN);
    ub_report_record_begin(&b, 1788950400);
    ub_report_add_field_i64(&b, "temperature", 1);
    ub_report_record_end(&b);
    int n = ub_report_end(&b);
    UB_CHECK(n > 0);

    UB_CHECK_EQ_INT(do_post("/api/v1/data/report", req, "not-a-real-token", &status, resp,
                            sizeof(resp), &resp_len),
                    0);
    UB_CHECK_EQ_INT(status, 401);

    int code;
    char msg[128];
    UB_CHECK_EQ_INT(ub_parse_error_response(resp, &code, msg, sizeof(msg)), 0);
    UB_CHECK_EQ_INT(code, UB_CODE_TOKEN_INVALID);
}

static void test_malformed_body_rejected(void) {
    char resp[512];
    int status;
    size_t resp_len;

    UB_CHECK_EQ_INT(do_post("/api/v1/auth/time", "{not valid json", NULL, &status, resp,
                            sizeof(resp), &resp_len),
                    0);
    UB_CHECK_EQ_INT(status, 400);

    int code;
    char msg[128];
    UB_CHECK_EQ_INT(ub_parse_error_response(resp, &code, msg, sizeof(msg)), 0);
    UB_CHECK_EQ_INT(code, UB_CODE_MALFORMED_BODY);
}

int main(int argc, char **argv) {
    g_host = argc > 1 ? argv[1] : "127.0.0.1";
    g_port = argc > 2 ? atoi(argv[2]) : 8080;
    ub_sockets_transport_init(&g_tr);

    fprintf(stderr, "running integration tests against %s:%d\n", g_host, g_port);

    test_full_lifecycle();
    test_report_rejects_bad_token();
    test_malformed_body_rejected();

    UB_TEST_SUMMARY();
}
