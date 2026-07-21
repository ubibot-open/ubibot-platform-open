/* Transport seam: ub_protocol.c only builds/parses JSON, it never touches a
 * socket. A real firmware target implements these three functions on top of
 * whatever it has (vendor TLS stack, an AT-command modem, lwIP, ...);
 * ub_transport_sockets.c is a plain-HTTP implementation over BSD/Winsock
 * sockets used for the host simulator (Linux/Windows) — see
 * simulation/freertos_port/README.md for the lwIP mapping. */
#ifndef UB_TRANSPORT_H
#define UB_TRANSPORT_H

#include <stddef.h>
#include <stdint.h>

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

/* Same contract as post, but a GET with no body — used for the config poll
 * endpoint (protocol §7.1). */
typedef int (*ub_transport_get_fn)(void *user_ctx, const char *host, int port, const char *path,
                                    const char *token_header_value, int *status, char *resp_buf,
                                    size_t resp_cap, size_t *resp_len);

/* Called once per chunk of body data received during a streaming download,
 * in order, with no re-delivery. Return 0 to keep going, non-zero to abort
 * the download (download_fn then returns non-zero too). This is how OTA
 * firmware images are received without ever holding the whole image in
 * memory. */
typedef int (*ub_transport_chunk_fn)(void *chunk_ctx, const uint8_t *data, size_t len);

/* Streaming GET with optional resume support. If range_start > 0, sends
 * "Range: bytes=<range_start>-" so an interrupted OTA download can resume
 * instead of restarting; the server is expected to reply 206 with the
 * remaining bytes (or 200 with the full body if it doesn't support Range,
 * in which case the caller must detect that from *status and restart from
 * offset 0 itself).
 *
 * *content_length is set from the response's Content-Length header (the
 * length of *this response's* body, i.e. the remaining bytes when
 * resuming) if present, else left at -1.
 *
 * Returns 0 only if the full response body was streamed through to
 * on_chunk; non-zero for any connection failure or an on_chunk abort. */
typedef int (*ub_transport_download_fn)(void *user_ctx, const char *host, int port,
                                         const char *path, const char *token_header_value,
                                         long range_start, int *status, long *content_length,
                                         ub_transport_chunk_fn on_chunk, void *chunk_ctx);

typedef struct {
    ub_transport_post_fn post;
    ub_transport_get_fn get;
    ub_transport_download_fn download;
    void *user_ctx;
} ub_transport_t;

#ifdef __cplusplus
}
#endif

#endif /* UB_TRANSPORT_H */
