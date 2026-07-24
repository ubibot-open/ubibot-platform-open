/* Host implementation of ub_platform.h for the Linux/Windows simulator.
 * Firmware provides an equivalent over FreeRTOS primitives (vTaskDelay,
 * a UART printf, ...); nothing that includes only ub_platform.h needs to
 * change. */
#ifndef _WIN32
/* clock_gettime/CLOCK_MONOTONIC and usleep need this feature-test macro
 * exposed under -std=c11's strict mode (glibc hides them otherwise). */
#define _POSIX_C_SOURCE 200809L
#endif

#include "ub_platform.h"

#include <stdarg.h>
#include <stdio.h>
#include <time.h>

#ifdef _WIN32
#include <windows.h>
#else
#include <unistd.h>
#endif

/* The host's own wall clock is already correct, but we still honor
 * ub_platform_set_time via an offset so the simulator behaves like a
 * device with no RTC that only trusts the server's clock (learned via
 * /auth/time or the "t" field of a report response). */
static int64_t g_time_offset = 0;

int64_t ub_platform_now(void) { return (int64_t)time(NULL) + g_time_offset; }

void ub_platform_set_time(int64_t unix_seconds) {
    g_time_offset = unix_seconds - (int64_t)time(NULL);
}

uint64_t ub_platform_monotonic_ms(void) {
#ifdef _WIN32
    /* QueryPerformanceCounter rather than GetTickCount64: it's available on
     * every Windows version's <windows.h> without extra feature-test
     * defines (some older mingw-w64 header sets gate GetTickCount64 behind
     * a WINAPI partition macro that isn't set for plain desktop builds),
     * and it doesn't wrap around every 49 days the way GetTickCount does. */
    static LARGE_INTEGER freq;
    static int have_freq = 0;
    if (!have_freq) {
        QueryPerformanceFrequency(&freq);
        have_freq = 1;
    }
    LARGE_INTEGER now;
    QueryPerformanceCounter(&now);
    return (uint64_t)((now.QuadPart * 1000) / freq.QuadPart);
#else
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (uint64_t)ts.tv_sec * 1000ULL + (uint64_t)(ts.tv_nsec / 1000000L);
#endif
}

void ub_platform_sleep_ms(uint32_t ms) {
#ifdef _WIN32
    Sleep(ms);
#else
    struct timespec ts;
    ts.tv_sec = ms / 1000;
    ts.tv_nsec = (long)(ms % 1000) * 1000000L;
    nanosleep(&ts, NULL);
#endif
}

void ub_platform_log(const char *fmt, ...) {
    fprintf(stderr, "[%llu] ", (unsigned long long)ub_platform_monotonic_ms());
    va_list ap;
    va_start(ap, fmt);
    vfprintf(stderr, fmt, ap);
    va_end(ap);
    fprintf(stderr, "\n");
}
