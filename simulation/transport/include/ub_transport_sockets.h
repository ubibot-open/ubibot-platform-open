/* Host-side (Linux/Windows) implementation of ub_transport_t over plain
 * BSD/Winsock sockets. Not for use in the firmware build — see
 * simulation/freertos_port/README.md for the lwIP-based replacement, which
 * implements the exact same three function pointers. */
#ifndef UB_TRANSPORT_SOCKETS_H
#define UB_TRANSPORT_SOCKETS_H

#include "ub_transport.h"

#ifdef __cplusplus
extern "C" {
#endif

void ub_sockets_transport_init(ub_transport_t *tr);

#ifdef __cplusplus
}
#endif

#endif /* UB_TRANSPORT_SOCKETS_H */
