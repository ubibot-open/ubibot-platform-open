/* Persistence HAL: a minimal named-blob store standing in for a flash
 * sector / NVS namespace. ub_device.c uses one slot ("state") to survive a
 * restart without re-activating from scratch. simulation/app/src/
 * ub_storage_file.c implements this as a single file under the device's
 * data directory on the host; firmware replaces it with a real NVS/flash
 * driver behind the same two functions. */
#ifndef UB_STORAGE_H
#define UB_STORAGE_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Sets the directory the host implementation stores blobs under (one file
 * per name). Call once at startup before the first load/save. Firmware's
 * NVS/flash implementation has no equivalent -- it can make this a no-op. */
void ub_storage_set_base_dir(const char *dir);

/* Loads up to `cap` bytes previously saved under `name` into `buf`.
 * Returns the number of bytes actually loaded (0 if nothing was ever saved
 * under this name), or -1 on a read error. */
int ub_storage_load(const char *name, void *buf, size_t cap);

/* Persists exactly `len` bytes under `name`, overwriting whatever was
 * there before. Returns 0 on success, -1 on error. */
int ub_storage_save(const char *name, const void *buf, size_t len);

#ifdef __cplusplus
}
#endif

#endif /* UB_STORAGE_H */
