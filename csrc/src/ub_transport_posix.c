/* Plain-HTTP transport over BSD sockets (or Winsock on Windows), used only
 * to exercise the protocol codec against a real running server in tests.
 * Firmware targets should replace this file with their own vendor
 * TLS/socket/AT-command stack behind the same ub_transport_t interface. */
#include "ub_transport_posix.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
#include <winsock2.h>
#include <ws2tcpip.h>
#pragma comment(lib, "ws2_32.lib")
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

static int ub_posix_post(void *user_ctx, const char *host, int port, const char *path,
                          const char *body, size_t body_len, const char *token_header_value,
                          int *status, char *resp_buf, size_t resp_cap, size_t *resp_len) {
    (void)user_ctx;

#ifdef _WIN32
    static int wsa_ready = 0;
    if (!wsa_ready) {
        WSADATA wsa;
        if (WSAStartup(MAKEWORD(2, 2), &wsa) != 0) return -1;
        wsa_ready = 1;
    }
#endif

    char port_str[8];
    snprintf(port_str, sizeof(port_str), "%d", port);

    struct addrinfo hints;
    memset(&hints, 0, sizeof(hints));
    hints.ai_family = AF_INET;
    hints.ai_socktype = SOCK_STREAM;

    struct addrinfo *res = NULL;
    if (getaddrinfo(host, port_str, &hints, &res) != 0) {
        return -1;
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
    if (fd == UB_INVALID_SOCKET) return -1;

    char header[512];
    int header_len;
    if (token_header_value && token_header_value[0] != '\0') {
        header_len = snprintf(header, sizeof(header),
                               "POST %s HTTP/1.1\r\n"
                               "Host: %s\r\n"
                               "Content-Type: application/json\r\n"
                               "X-IoT-Token: %s\r\n"
                               "Content-Length: %zu\r\n"
                               "Connection: close\r\n"
                               "\r\n",
                               path, host, token_header_value, body_len);
    } else {
        header_len = snprintf(header, sizeof(header),
                               "POST %s HTTP/1.1\r\n"
                               "Host: %s\r\n"
                               "Content-Type: application/json\r\n"
                               "Content-Length: %zu\r\n"
                               "Connection: close\r\n"
                               "\r\n",
                               path, host, body_len);
    }
    if (header_len < 0 || (size_t)header_len >= sizeof(header)) {
        UB_CLOSESOCK(fd);
        return -1;
    }

    if (send(fd, header, header_len, 0) != header_len) {
        UB_CLOSESOCK(fd);
        return -1;
    }
    size_t sent = 0;
    while (sent < body_len) {
        int n = send(fd, body + sent, (int)(body_len - sent), 0);
        if (n <= 0) {
            UB_CLOSESOCK(fd);
            return -1;
        }
        sent += (size_t)n;
    }

    /* We asked for Connection: close, so reading until the peer closes the
     * socket is a correct (if not maximally efficient) way to get the
     * whole response without needing to track Content-Length ourselves. */
    static char raw[8192];
    size_t raw_len = 0;
    for (;;) {
        if (raw_len >= sizeof(raw) - 1) break;
        int n = recv(fd, raw + raw_len, (int)(sizeof(raw) - 1 - raw_len), 0);
        if (n <= 0) break;
        raw_len += (size_t)n;
    }
    UB_CLOSESOCK(fd);
    raw[raw_len] = '\0';

    const char *sp = strchr(raw, ' ');
    if (!sp) return -1;
    int parsed_status = atoi(sp + 1);

    const char *body_start = strstr(raw, "\r\n\r\n");
    if (!body_start) return -1;
    body_start += 4;

    size_t body_actual_len = raw_len - (size_t)(body_start - raw);
    size_t copy_len = (resp_cap == 0) ? 0 : (body_actual_len < resp_cap - 1 ? body_actual_len : resp_cap - 1);
    if (copy_len > 0) memcpy(resp_buf, body_start, copy_len);
    if (resp_cap > 0) resp_buf[copy_len] = '\0';

    *status = parsed_status;
    *resp_len = copy_len;
    return 0;
}

void ub_posix_transport_init(ub_transport_t *tr) {
    tr->post = ub_posix_post;
    tr->user_ctx = NULL;
}
