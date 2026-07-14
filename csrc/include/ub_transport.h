/* Transport seam: ub_protocol.c only builds/parses JSON, it never touches a
 * socket. A real firmware target implements `post` on top of whatever it
 * has (vendor TLS stack, an AT-command modem, lwIP, ...); ub_transport_posix.c
 * is a plain-HTTP implementation over BSD/Winsock sockets used for host-side
 * testing (see csrc/test/test_integration.c). */
#ifndef UB_TRANSPORT_H
#define UB_TRANSPORT_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Performs a single POST request and blocks for the response.
 *
 * token_header_value: if non-NULL and non-empty, sent as the X-IoT-Token
 * header.
 *
 * On return, *status is the HTTP status code and resp_buf holds the
 * response body (NUL-terminated, truncated to resp_cap-1 bytes if
 * necessary) with *resp_len set to the copied length.
 *
 * Returns 0 if the request/response round trip completed at the transport
 * level (regardless of what HTTP status came back — a 4xx/5xx is not a
 * transport failure). Returns non-zero only for a connection-level failure
 * (DNS, connect, send/recv error). */
typedef int (*ub_transport_post_fn)(void *user_ctx, const char *host, int port, const char *path,
                                     const char *body, size_t body_len,
                                     const char *token_header_value, int *status, char *resp_buf,
                                     size_t resp_cap, size_t *resp_len);

typedef struct {
    ub_transport_post_fn post;
    void *user_ctx;
} ub_transport_t;

#ifdef __cplusplus
}
#endif

#endif /* UB_TRANSPORT_H */
