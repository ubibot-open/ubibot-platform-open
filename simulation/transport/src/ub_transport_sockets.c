/* Plain-HTTP transport over BSD sockets (or Winsock on Windows). This is
 * the *only* file in the whole simulation/ tree that is not directly
 * reusable on real FreeRTOS firmware as-is: it exists purely so the host
 * simulator can talk to the server without a TLS stack. A firmware target
 * replaces this file with one that speaks the same ub_transport_t contract
 * on top of lwIP (whose socket API is intentionally BSD-compatible, so most
 * of this code ports with only the ifdef block below changing) or a vendor
 * TLS/AT-modem stack. See simulation/freertos_port/README.md. */
#ifndef _WIN32
/* getaddrinfo/freeaddrinfo/struct addrinfo need this feature-test macro
 * exposed under -std=c11's strict mode (glibc hides them otherwise). */
#define _POSIX_C_SOURCE 200809L
#endif

#include "ub_transport_sockets.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
#include <winsock2.h>
#include <ws2tcpip.h>
#ifdef _MSC_VER
#pragma comment(lib, "ws2_32.lib") /* MinGW/GCC link ws2_32 via -lws2_32 instead */
#endif
typedef SOCKET ub_sock_t;
#define UB_CLOSESOCK closesocket
#define UB_INVALID_SOCKET INVALID_SOCKET
#else
#include <netdb.h>
#include <sys/socket.h>
#include <unistd.h>
typedef int ub_sock_t;
#define UB_CLOSESOCK close
#define UB_INVALID_SOCKET (-1)
#endif

static void ensure_wsa_ready(void) {
#ifdef _WIN32
    static int wsa_ready = 0;
    if (!wsa_ready) {
        WSADATA wsa;
        if (WSAStartup(MAKEWORD(2, 2), &wsa) == 0) wsa_ready = 1;
    }
#endif
}

static ub_sock_t connect_to(const char *host, int port) {
    ensure_wsa_ready();

    char port_str[8];
    snprintf(port_str, sizeof(port_str), "%d", port);

    struct addrinfo hints;
    memset(&hints, 0, sizeof(hints));
    hints.ai_family = AF_INET;
    hints.ai_socktype = SOCK_STREAM;

    struct addrinfo *res = NULL;
    if (getaddrinfo(host, port_str, &hints, &res) != 0) {
        return UB_INVALID_SOCKET;
    }

    ub_sock_t fd = UB_INVALID_SOCKET;
    for (struct addrinfo *p = res; p != NULL; p = p->ai_next) {
        fd = socket(p->ai_family, p->ai_socktype, p->ai_protocol);
        if (fd == UB_INVALID_SOCKET) continue;
        if (connect(fd, p->ai_addr, (int)p->ai_addrlen) == 0) break;
        UB_CLOSESOCK(fd);
        fd = UB_INVALID_SOCKET;
    }
    freeaddrinfo(res);
    return fd;
}

static int send_all(ub_sock_t fd, const char *data, size_t len) {
    size_t sent = 0;
    while (sent < len) {
        int n = send(fd, data + sent, (int)(len - sent), 0);
        if (n <= 0) return -1;
        sent += (size_t)n;
    }
    return 0;
}

/* Reads a full HTTP response (status line + headers + whole body) into a
 * bounded static buffer, relying on the server honoring our
 * "Connection: close" request to signal end-of-body. Both device endpoints
 * only ever exchange a small JSON body that comfortably fits. */
static int recv_full_response(ub_sock_t fd, int *status, char *resp_buf, size_t resp_cap,
                               size_t *resp_len) {
    static char raw[8192];
    size_t raw_len = 0;
    for (;;) {
        if (raw_len >= sizeof(raw) - 1) break;
        int n = recv(fd, raw + raw_len, (int)(sizeof(raw) - 1 - raw_len), 0);
        if (n <= 0) break;
        raw_len += (size_t)n;
    }
    raw[raw_len] = '\0';

    const char *sp = strchr(raw, ' ');
    if (!sp) return -1;
    int parsed_status = atoi(sp + 1);

    const char *body_start = strstr(raw, "\r\n\r\n");
    if (!body_start) return -1;
    body_start += 4;

    size_t body_actual_len = raw_len - (size_t)(body_start - raw);
    size_t copy_len =
        (resp_cap == 0) ? 0 : (body_actual_len < resp_cap - 1 ? body_actual_len : resp_cap - 1);
    if (copy_len > 0) memcpy(resp_buf, body_start, copy_len);
    if (resp_cap > 0) resp_buf[copy_len] = '\0';

    *status = parsed_status;
    *resp_len = copy_len;
    return 0;
}

static int ub_sock_post(void *user_ctx, const char *host, int port, const char *path,
                         const char *body, size_t body_len, int *status, char *resp_buf,
                         size_t resp_cap, size_t *resp_len) {
    (void)user_ctx;

    ub_sock_t fd = connect_to(host, port);
    if (fd == UB_INVALID_SOCKET) return -1;

    char header[512];
    int header_len = snprintf(header, sizeof(header),
                               "POST %s HTTP/1.1\r\n"
                               "Host: %s\r\n"
                               "Content-Type: application/json\r\n"
                               "Content-Length: %zu\r\n"
                               "Connection: close\r\n"
                               "\r\n",
                               path, host, body_len);
    if (header_len < 0 || (size_t)header_len >= sizeof(header)) {
        UB_CLOSESOCK(fd);
        return -1;
    }

    if (send_all(fd, header, (size_t)header_len) != 0 || send_all(fd, body, body_len) != 0) {
        UB_CLOSESOCK(fd);
        return -1;
    }

    int rc = recv_full_response(fd, status, resp_buf, resp_cap, resp_len);
    UB_CLOSESOCK(fd);
    return rc;
}

void ub_sockets_transport_init(ub_transport_t *tr) {
    tr->post = ub_sock_post;
    tr->user_ctx = NULL;
}
