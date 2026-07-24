/* Sensor HAL: stands in for real ADC/I2C/Modbus reads. The simulated
 * implementation (ub_sensors_sim.c) generates plausible drifting values so
 * the report stream looks like a real weather-station device; a firmware
 * target replaces the bodies with actual driver reads behind the same
 * three signatures.
 *
 * These three map onto the protocol's conventional field1/field2/field3
 * (temperature/humidity/light, protocol §5). There is no generic "custom
 * probe" reader any more -- field4-20 are for whoever adds more sensors and
 * are not exercised by this reference device. */
#ifndef UB_SENSORS_H
#define UB_SENSORS_H

#ifdef __cplusplus
extern "C" {
#endif

double ub_sensor_read_temperature(void); /* field1, degrees Celsius */
double ub_sensor_read_humidity(void);    /* field2, % relative humidity */
double ub_sensor_read_light(void);       /* field3, illuminance in lux */

#ifdef __cplusplus
}
#endif

#endif /* UB_SENSORS_H */
