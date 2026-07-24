/* SKELETON -- not compiled, not part of the build. Reference for
 * implementing simulation/transport/include/ub_transport.h on real
 * FreeRTOS+lwIP firmware. See simulation/freertos_port/README.md.
 *
 * lwIP's socket API (when built with LWIP_SOCKET=1) is intentionally
 * BSD-socket compatible, so this file's structure -- and in many places
 * its exact code -- mirrors simulation/transport/src/ub_transport_sockets.c's
 * POSIX branch (#else side of its `#ifdef _WIN32`) almost line for line.
 * The main real differences on a real target are usually: (1) swapping
 * plain sockets for a TLS-wrapped connection if the server requires HTTPS,
 * and (2) bounding buffer sizes much more tightly than the host version's
 * 8KB static buffers, to fit MCU RAM. */
#include "ub_transport.h"

#include <string.h>

#include "lwip/sockets.h"
#include "lwip/netdb.h"
/* TODO: if the server requires TLS, include your TLS wrapper here instead
 * (mbedTLS is the common choice on FreeRTOS) and wrap send/recv below. */

#define UB_LWIP_HDR_BUF 512
#define UB_LWIP_RESP_BUF 2048 /* smaller than the host's 8KB: tune to your RAM budget */

static int connect_to(const char *host, int port) {
    struct sockaddr_in addr;
    /* TODO: resolve `host` via lwip_gethostbyname or netconn_gethostbyname
     * if it's a DNS name; if it's always a fixed IP for your deployment,
     * inet_pton/ipaddr_aton is enough and avoids pulling in a resolver. */
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_port = lwip_htons((u16_t)port);
    /* addr.sin_addr.s_addr = ...; TODO */

    int fd = lwip_socket(AF_INET, SOCK_STREAM, 0);
    if (fd < 0) return -1;
    if (lwip_connect(fd, (struct sockaddr *)&addr, sizeof(addr)) != 0) {
        lwip_close(fd);
        return -1;
    }
    return fd;
}

static int send_all(int fd, const char *data, size_t len) {
    size_t sent = 0;
    while (sent < len) {
        int n = lwip_send(fd, data + sent, len - sent, 0);
        if (n <= 0) return -1;
        sent += (size_t)n;
    }
    return 0;
}

/* Follows exactly the same shape as ub_transport_sockets.c's
 * ub_sock_post -- only the socket calls change (lwip_* instead of the bare
 * libc names, which on lwIP's socket API are the same functions under a
 * prefix). Refer to that file for the full header-building and
 * response-parsing logic; it's omitted here to avoid duplicating ~80 lines
 * that don't change. There is no token/auth header to send -- the protocol
 * has no session token, just the plaintext pid+sn already in the JSON
 * body. */
static int ub_lwip_post(void *user_ctx, const char *host, int port, const char *path,
                         const char *body, size_t body_len, int *status, char *resp_buf,
                         size_t resp_cap, size_t *resp_len) {
    (void)user_ctx;
    (void)host; (void)port; (void)path; (void)body; (void)body_len;
    (void)status; (void)resp_buf; (void)resp_cap; (void)resp_len;
    /* TODO: port the body of ub_sock_post() from ub_transport_sockets.c,
     * using connect_to/send_all above and lwip_recv() in place of recv(). */
    return -1;
}

void ub_lwip_transport_init(ub_transport_t *tr) {
    tr->post = ub_lwip_post;
    tr->user_ctx = NULL;
}
