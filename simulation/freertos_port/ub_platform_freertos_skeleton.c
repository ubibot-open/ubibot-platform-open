/* SKELETON -- not compiled, not part of the build. Reference for
 * implementing simulation/app/include/ub_platform.h on real FreeRTOS
 * firmware. Copy this file's role (not necessarily its exact content) into
 * your project as ub_platform_freertos.c, filling in the TODOs for your
 * specific BSP/SDK. See simulation/freertos_port/README.md.
 */
#include "ub_platform.h"

#include <stdarg.h>
#include "FreeRTOS.h"
#include "task.h"
/* TODO: #include your RTC / SNTP driver header */
/* TODO: #include your UART/RTT logging driver header */

/* Mirrors ub_platform_host.c's g_time_offset trick: most MCUs have no
 * battery-backed RTC accurate enough to trust on its own, so the device
 * tracks an offset from its own tick count and only trusts the *server's*
 * clock (learned via /auth/time or the "t" field of a report response, see
 * ub_device.c). If your board does have a trustworthy RTC, ub_platform_now()
 * can read it directly instead and ub_platform_set_time() can become a
 * no-op (or still correct RTC drift, at your discretion). */
static int64_t g_time_offset_sec = 0;

int64_t ub_platform_now(void) {
    /* TODO: base this off something monotonic-since-boot in real seconds,
     * e.g. (xTaskGetTickCount() * portTICK_PERIOD_MS) / 1000, plus the
     * offset below. */
    int64_t boot_relative_sec = 0; /* TODO */
    return boot_relative_sec + g_time_offset_sec;
}

void ub_platform_set_time(int64_t unix_seconds) {
    int64_t boot_relative_sec = 0; /* TODO: same basis as ub_platform_now above */
    g_time_offset_sec = unix_seconds - boot_relative_sec;
}

uint64_t ub_platform_monotonic_ms(void) {
    return (uint64_t)xTaskGetTickCount() * portTICK_PERIOD_MS;
}

void ub_platform_sleep_ms(uint32_t ms) {
    vTaskDelay(pdMS_TO_TICKS(ms));
}

void ub_platform_log(const char *fmt, ...) {
    /* TODO: replace with your UART/RTT printf. Keep it non-blocking or
     * bounded if this is called from a time-sensitive task. */
    va_list ap;
    va_start(ap, fmt);
    /* vprintf_uart(fmt, ap); */
    va_end(ap);
}
