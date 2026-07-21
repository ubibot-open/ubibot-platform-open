/* Simulated sensor readings: a slow random walk around a plausible
 * baseline, so a report stream looks like a real weather-station device
 * instead of a flat line. Firmware replaces each function body with an
 * actual ADC/I2C/Modbus read; the signatures (and therefore every caller
 * in ub_device.c) don't change. */
#include "ub_sensors.h"

#include <stdlib.h>

static double g_temperature = 25.0;
static double g_humidity = 55.0;
static int g_seeded = 0;

static double step(double value, double min, double max, double max_delta) {
    double delta = ((double)rand() / (double)RAND_MAX * 2.0 - 1.0) * max_delta;
    value += delta;
    if (value < min) value = min;
    if (value > max) value = max;
    return value;
}

static void seed_once(void) {
    if (!g_seeded) {
        srand((unsigned int)42);
        g_seeded = 1;
    }
}

double ub_sensor_read_temperature(void) {
    seed_once();
    g_temperature = step(g_temperature, -10.0, 45.0, 0.3);
    return g_temperature;
}

double ub_sensor_read_humidity(void) {
    seed_once();
    g_humidity = step(g_humidity, 0.0, 100.0, 1.0);
    return g_humidity;
}

double ub_sensor_read_probe(const char *pid, double scale, double offset) {
    (void)pid;
    seed_once();
    /* A generic drifting raw value, scaled/offset the same way a real
     * Modbus register read would be after set_probe configures it. */
    static double raw = 500.0;
    raw = step(raw, 0.0, 4095.0, 20.0);
    return raw * scale + offset;
}
