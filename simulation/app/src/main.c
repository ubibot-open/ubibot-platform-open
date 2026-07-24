/* Host entry point for the IoT device simulator. Parses a handful of
 * command-line options, wires up the sockets transport, and then runs the
 * device's main loop -- the loop itself is exactly what a FreeRTOS task
 * body would look like on real firmware (see
 * simulation/freertos_port/README.md).
 *
 * Usage:
 *   ub_device_sim [--host H] [--port P] [--pid PID] [--sn SN] [--tick-ms MS]
 *
 * Defaults match the demo device seeded by cmd/server/main.go, so running
 * with no arguments against a locally started server (`go run ./cmd/server`)
 * works out of the box.
 *
 * There is no --secret flag any more: the protocol has no device secret,
 * signature, or activation step at all (see docs/UbiBot开放平台硬件通信协议.md).
 * There is also no --data-dir flag: nothing needs to survive a restart
 * (no session token, no server-pushed config), so the simulator has no
 * persisted state to store anywhere.
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "ub_device.h"
#include "ub_platform.h"
#include "ub_transport_sockets.h"

static void print_usage(const char *prog) {
    fprintf(stderr, "usage: %s [--host H] [--port P] [--pid PID] [--sn SN] [--tick-ms MS]\n",
            prog);
}

int main(int argc, char **argv) {
    ub_device_config_t cfg;
    memset(&cfg, 0, sizeof(cfg));
    snprintf(cfg.pid, sizeof(cfg.pid), "ubibot_open_dev_v1");
    snprintf(cfg.sn, sizeof(cfg.sn), "sn_ws1_20001_1");
    snprintf(cfg.server_host, sizeof(cfg.server_host), "127.0.0.1");
    cfg.server_port = 8080;

    uint32_t tick_ms = 1000;

    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "--host") == 0 && i + 1 < argc) {
            snprintf(cfg.server_host, sizeof(cfg.server_host), "%s", argv[++i]);
        } else if (strcmp(argv[i], "--port") == 0 && i + 1 < argc) {
            cfg.server_port = atoi(argv[++i]);
        } else if (strcmp(argv[i], "--pid") == 0 && i + 1 < argc) {
            snprintf(cfg.pid, sizeof(cfg.pid), "%s", argv[++i]);
        } else if (strcmp(argv[i], "--sn") == 0 && i + 1 < argc) {
            snprintf(cfg.sn, sizeof(cfg.sn), "%s", argv[++i]);
        } else if (strcmp(argv[i], "--tick-ms") == 0 && i + 1 < argc) {
            tick_ms = (uint32_t)atoi(argv[++i]);
        } else if (strcmp(argv[i], "--help") == 0 || strcmp(argv[i], "-h") == 0) {
            print_usage(argv[0]);
            return 0;
        } else {
            fprintf(stderr, "unknown option: %s\n", argv[i]);
            print_usage(argv[0]);
            return 1;
        }
    }

    ub_platform_log("ubibot device simulator starting: pid=%s sn=%s server=%s:%d", cfg.pid,
                     cfg.sn, cfg.server_host, cfg.server_port);

    ub_transport_t transport;
    ub_sockets_transport_init(&transport);

    ub_device_t dev;
    ub_device_init(&dev, &cfg, &transport);

    /* This loop is the entire "application task": on FreeRTOS it is the
     * body of the task created for the device, with ub_platform_sleep_ms
     * mapped to vTaskDelay. Nothing here is host-specific. */
    while (dev.running) {
        ub_device_tick(&dev);
        ub_platform_sleep_ms(tick_ms);
    }
    return 0;
}
