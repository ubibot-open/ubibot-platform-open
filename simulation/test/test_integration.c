/* Integration test: drives the C client codec over a real TCP connection
 * against a *running* instance of cmd/server. Exercises the full device
 * lifecycle for the simplified protocol: time sync -> report (auto-creates
 * the device) -> a couple of error paths -- the things a real device does
 * that a pure unit test can't reach without a socket.
 *
 * Requires a live server: `go run ./cmd/server` from the repo root, then
 * run this binary (built via `make build/test_integration` or the
 * `test_integration` CMake target -- neither is wired into `make test` /
 * ctest, since there is no server to point at in this environment).
 *
 * Usage: test_integration [host] [port]
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include "ub_protocol.h"
#include "ub_test.h"
#include "ub_transport_sockets.h"

/* Any pid/sn works -- the server auto-creates the device on first report,
 * there is no pre-provisioning step any more. A random-ish sn keeps repeat
 * runs of this test from colliding with a device a previous run created. */
#define PID "ubibot_open_dev_v1"

static ub_transport_t g_tr;
static const char *g_host;
static int g_port;
static char g_sn[64];

static int do_post(const char *path, const char *body, int *status, char *resp, size_t resp_cap,
                    size_t *resp_len) {
    return g_tr.post(g_tr.user_ctx, g_host, g_port, path, body, strlen(body), status, resp,
                      resp_cap, resp_len);
}

static void test_time_sync(void) {
    ub_identity_t id = {PID, g_sn};
    char req[128];
    char resp[256];
    int status;
    size_t resp_len;

    int n = ub_build_time_request(&id, req, sizeof(req));
    UB_CHECK(n > 0);
    UB_CHECK_EQ_INT(do_post("/api/v1/auth/time", req, &status, resp, sizeof(resp), &resp_len), 0);
    UB_CHECK_EQ_INT(status, 200);

    ub_time_response_t tr;
    UB_CHECK_EQ_INT(ub_parse_time_response(resp, &tr), 0);
    UB_CHECK_EQ_INT(tr.c, UB_CODE_OK);
    UB_CHECK(tr.t > 0);
}

static void test_report_auto_creates_device(void) {
    char buf[512];
    char resp[512];
    int status;
    size_t resp_len;

    int64_t now = (int64_t)time(NULL);

    ub_report_builder_t b;
    ub_report_begin(&b, buf, sizeof(buf), PID, g_sn, now);
    ub_report_payload_begin(&b, now);
    ub_report_add_field(&b, 1, 25.6);
    ub_report_add_field(&b, 2, 60.2);
    ub_report_payload_end(&b);
    ub_report_payload_begin(&b, now + 600);
    ub_report_add_field(&b, 1, 25.8);
    ub_report_add_field(&b, 3, 812);
    ub_report_payload_end(&b);
    int n = ub_report_end(&b);
    UB_CHECK(n > 0);

    UB_CHECK_EQ_INT(do_post("/api/v1/data/report", buf, &status, resp, sizeof(resp), &resp_len),
                    0);
    UB_CHECK_EQ_INT(status, 200);

    ub_report_response_t rr;
    UB_CHECK_EQ_INT(ub_parse_report_response(resp, &rr), 0);
    UB_CHECK_EQ_INT(rr.c, UB_CODE_OK);

    /* Response must actually be small -- the whole point of this protocol's
     * "no cfg/cmd, just {c,t}" response shape. */
    UB_CHECK(resp_len < 256);
}

static void test_report_timestamp_out_of_window_rejected(void) {
    char buf[256], resp[256];
    int status;
    size_t resp_len;

    int64_t stale = (int64_t)time(NULL) - 3600; /* 1 hour ago, well past +-5min */

    ub_report_builder_t b;
    ub_report_begin(&b, buf, sizeof(buf), PID, g_sn, stale);
    ub_report_payload_begin(&b, stale);
    ub_report_add_field(&b, 1, 20.0);
    ub_report_payload_end(&b);
    int n = ub_report_end(&b);
    UB_CHECK(n > 0);

    UB_CHECK_EQ_INT(do_post("/api/v1/data/report", buf, &status, resp, sizeof(resp), &resp_len),
                    0);
    UB_CHECK_EQ_INT(status, 400);

    int code;
    char msg[128];
    UB_CHECK_EQ_INT(ub_parse_error_response(resp, &code, msg, sizeof(msg)), 0);
    UB_CHECK_EQ_INT(code, UB_CODE_TIMESTAMP_OUT_OF_RANGE);
}

static void test_malformed_body_rejected(void) {
    char resp[256];
    int status;
    size_t resp_len;

    UB_CHECK_EQ_INT(
        do_post("/api/v1/auth/time", "{not valid json", &status, resp, sizeof(resp), &resp_len),
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
    snprintf(g_sn, sizeof(g_sn), "sn_test_integration_%ld", (long)time(NULL));
    ub_sockets_transport_init(&g_tr);

    fprintf(stderr, "running integration tests against %s:%d (sn=%s)\n", g_host, g_port, g_sn);

    test_time_sync();
    test_report_auto_creates_device();
    test_report_timestamp_out_of_window_rejected();
    test_malformed_body_rejected();

    UB_TEST_SUMMARY();
}
