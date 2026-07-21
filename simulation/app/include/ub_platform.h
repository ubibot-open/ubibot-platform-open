/* Platform HAL: the small set of OS-level primitives ub_device.c needs
 * (clock, sleep, reboot, logging). simulation/app/src/ub_platform_host.c
 * implements this over the host's libc for Linux/Windows; a real firmware
 * target implements the same five functions over FreeRTOS (vTaskDelay,
 * esp_restart/NVIC_SystemReset, a UART printf, ...) — see
 * simulation/freertos_port/README.md. Nothing above this header (device
 * state machine, protocol codec) needs to change to run on either. */
#ifndef UB_PLATFORM_H
#define UB_PLATFORM_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Best-known wall-clock unix seconds. */
int64_t ub_platform_now(void);

/* Corrects the platform's notion of wall-clock time once the device learns
 * the server's time (via /auth/time or /auth/activate) -- meaningful on
 * firmware with no battery-backed RTC; can be a no-op on a host that
 * already has a correct system clock. */
void ub_platform_set_time(int64_t unix_seconds);

/* Monotonic milliseconds since an arbitrary fixed point (typically boot).
 * Never adjusted by ub_platform_set_time. Used for the warm-boot
 * activation replay-window check (protocol §4) and internal timers. */
uint64_t ub_platform_monotonic_ms(void);

void ub_platform_sleep_ms(uint32_t ms);

/* Reboots the device. The host simulator exits the process; firmware
 * performs a real MCU reset. Does not return. */
void ub_platform_reboot(void);

/* Line-oriented diagnostic log, analogous to a UART/console print. */
void ub_platform_log(const char *fmt, ...);

#ifdef __cplusplus
}
#endif

#endif /* UB_PLATFORM_H */
