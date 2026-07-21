/* Host implementation of ub_storage.h: one flat file per name under a base
 * directory, standing in for a flash sector / NVS namespace. Firmware
 * replaces this file with a real NVS/flash driver behind the same
 * load/save signatures. */
#include "ub_storage.h"

#include <stdio.h>
#include <string.h>

#define UB_STORAGE_PATH_MAX 512

static char g_base_dir[UB_STORAGE_PATH_MAX] = ".";

void ub_storage_set_base_dir(const char *dir) {
    if (!dir || dir[0] == '\0') return;
    snprintf(g_base_dir, sizeof(g_base_dir), "%s", dir);
}

static void build_path(const char *name, char *out, size_t out_cap) {
    snprintf(out, out_cap, "%s/%s.bin", g_base_dir, name);
}

int ub_storage_load(const char *name, void *buf, size_t cap) {
    char path[UB_STORAGE_PATH_MAX];
    build_path(name, path, sizeof(path));

    FILE *f = fopen(path, "rb");
    if (!f) return 0; /* nothing saved yet is not an error */

    size_t n = fread(buf, 1, cap, f);
    fclose(f);
    return (int)n;
}

int ub_storage_save(const char *name, const void *buf, size_t len) {
    char path[UB_STORAGE_PATH_MAX];
    build_path(name, path, sizeof(path));

    FILE *f = fopen(path, "wb");
    if (!f) return -1;

    size_t n = fwrite(buf, 1, len, f);
    fclose(f);
    return (n == len) ? 0 : -1;
}
