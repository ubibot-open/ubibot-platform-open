/* Sensor HAL: stands in for real ADC/I2C/Modbus reads. The simulated
 * implementation (ub_sensors_sim.c) generates plausible drifting values so
 * the report stream looks like a real weather-station device; a firmware
 * target replaces the bodies with actual driver reads behind the same
 * three signatures. */
#ifndef UB_SENSORS_H
#define UB_SENSORS_H

#ifdef __cplusplus
extern "C" {
#endif

/* Built-in sensors (protocol §5 / the device's "fe" enabled-sensor list). */
double ub_sensor_read_temperature(void);
double ub_sensor_read_humidity(void);

/* A custom probe reading identified by its configured pid (see
 * ub_probe_t in ub_device.h), already scaled/offset per its set_probe
 * configuration -- standing in for whatever a real Modbus/analog probe
 * driver would decode from `iface`/`proto`/`addr`. */
double ub_sensor_read_probe(const char *pid, double scale, double offset);

#ifdef __cplusplus
}
#endif

#endif /* UB_SENSORS_H */
