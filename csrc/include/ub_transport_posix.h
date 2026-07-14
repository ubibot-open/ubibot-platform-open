/* Host-only plain-HTTP transport (BSD sockets / Winsock), for tests. Not
 * meant to be linked into firmware. */
#ifndef UB_TRANSPORT_POSIX_H
#define UB_TRANSPORT_POSIX_H

#include "ub_transport.h"

#ifdef __cplusplus
extern "C" {
#endif

void ub_posix_transport_init(ub_transport_t *tr);

#ifdef __cplusplus
}
#endif

#endif /* UB_TRANSPORT_POSIX_H */
