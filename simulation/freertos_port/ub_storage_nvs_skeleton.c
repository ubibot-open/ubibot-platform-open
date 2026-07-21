/* SKELETON -- not compiled, not part of the build. Reference for
 * implementing simulation/app/include/ub_storage.h on real firmware, e.g.
 * ESP-IDF's NVS (used here as the illustrative example) or an equivalent
 * flash-backed key/value store. See simulation/freertos_port/README.md.
 */
#include "ub_storage.h"

/* TODO: #include "nvs.h" / "nvs_flash.h" or your platform's KV store header. */

/* ub_device.c only ever uses one blob, name="state", of a fixed sizeof()
 * (see ub_persisted_state_t in ub_device.c) -- there is no need for a
 * general-purpose filesystem here, just a single fixed-size record. */

void ub_storage_set_base_dir(const char *dir) {
    (void)dir; /* no equivalent concept for a flash-KV namespace; no-op */
}

int ub_storage_load(const char *name, void *buf, size_t cap) {
    (void)name;
    (void)buf;
    (void)cap;
    /* TODO:
     *   nvs_handle_t h;
     *   if (nvs_open("ubibot", NVS_READONLY, &h) != ESP_OK) return 0;
     *   size_t actual = cap;
     *   esp_err_t err = nvs_get_blob(h, name, buf, &actual);
     *   nvs_close(h);
     *   if (err == ESP_ERR_NVS_NOT_FOUND) return 0; // nothing saved yet, not an error
     *   if (err != ESP_OK) return -1;
     *   return (int)actual;
     */
    return 0;
}

int ub_storage_save(const char *name, const void *buf, size_t len) {
    (void)name;
    (void)buf;
    (void)len;
    /* TODO:
     *   nvs_handle_t h;
     *   if (nvs_open("ubibot", NVS_READWRITE, &h) != ESP_OK) return -1;
     *   esp_err_t err = nvs_set_blob(h, name, buf, len);
     *   if (err == ESP_OK) err = nvs_commit(h);
     *   nvs_close(h);
     *   return err == ESP_OK ? 0 : -1;
     */
    return -1;
}
