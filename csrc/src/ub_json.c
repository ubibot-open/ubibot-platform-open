#include "ub_json.h"

#include <stdlib.h>
#include <string.h>

static const char *skip_ws(const char *p) {
    while (*p == ' ' || *p == '\t' || *p == '\n' || *p == '\r') {
        p++;
    }
    return p;
}

/* p must point at the opening quote. Returns a pointer to the closing
 * quote, or NULL if the string is unterminated. */
static const char *find_string_end(const char *p) {
    p++; /* skip opening quote */
    while (*p != '\0') {
        if (*p == '\\') {
            if (p[1] == '\0') {
                return NULL;
            }
            p += 2;
            continue;
        }
        if (*p == '"') {
            return p;
        }
        p++;
    }
    return NULL;
}

const char *ub_json_skip_value(const char *p) {
    p = skip_ws(p);
    if (*p == '"') {
        const char *end = find_string_end(p);
        if (!end) return NULL;
        return skip_ws(end + 1);
    }
    if (*p == '{' || *p == '[') {
        char open = *p, close = (*p == '{') ? '}' : ']';
        int depth = 1;
        p++;
        while (depth > 0) {
            if (*p == '\0') return NULL;
            if (*p == '"') {
                const char *end = find_string_end(p);
                if (!end) return NULL;
                p = end + 1;
                continue;
            }
            if (*p == open) depth++;
            else if (*p == close) depth--;
            p++;
        }
        return skip_ws(p);
    }
    if (strncmp(p, "true", 4) == 0) return skip_ws(p + 4);
    if (strncmp(p, "false", 5) == 0) return skip_ws(p + 5);
    if (strncmp(p, "null", 4) == 0) return skip_ws(p + 4);

    /* number: -?digits(.digits)?([eE][+-]?digits)? */
    const char *start = p;
    if (*p == '-') p++;
    while (*p >= '0' && *p <= '9') p++;
    if (*p == '.') {
        p++;
        while (*p >= '0' && *p <= '9') p++;
    }
    if (*p == 'e' || *p == 'E') {
        p++;
        if (*p == '+' || *p == '-') p++;
        while (*p >= '0' && *p <= '9') p++;
    }
    if (p == start) return NULL; /* not a value at all */
    return skip_ws(p);
}

const char *ub_json_find_key(const char *obj, const char *key) {
    if (*obj != '{') return NULL;
    const char *p = skip_ws(obj + 1);
    if (*p == '}') return NULL;

    size_t key_len = strlen(key);
    for (;;) {
        p = skip_ws(p);
        if (*p != '"') return NULL;
        const char *key_start = p + 1;
        const char *key_end = find_string_end(p);
        if (!key_end) return NULL;

        p = skip_ws(key_end + 1);
        if (*p != ':') return NULL;
        p = skip_ws(p + 1);
        const char *value_start = p;

        int match = ((size_t)(key_end - key_start) == key_len) &&
                    (memcmp(key_start, key, key_len) == 0);

        p = ub_json_skip_value(value_start);
        if (!p) return NULL;

        if (match) return value_start;

        if (*p == ',') {
            p++;
            continue;
        }
        if (*p == '}') return NULL;
        return NULL; /* malformed */
    }
}

int ub_json_get_string(const char *val, char *out, size_t out_cap) {
    val = skip_ws(val);
    if (*val != '"' || out_cap == 0) return -1;
    const char *p = val + 1;
    size_t n = 0;

    while (*p != '"') {
        if (*p == '\0') return -1;
        char c = *p;
        if (c == '\\') {
            p++;
            switch (*p) {
                case '"': c = '"'; break;
                case '\\': c = '\\'; break;
                case '/': c = '/'; break;
                case 'n': c = '\n'; break;
                case 't': c = '\t'; break;
                case 'r': c = '\r'; break;
                default:
                    return -1; /* \uXXXX and friends: not needed for our field set */
            }
        }
        if (n + 1 >= out_cap) return -1; /* leave room for NUL */
        out[n++] = c;
        p++;
    }
    out[n] = '\0';
    return 0;
}

int ub_json_get_int64(const char *val, int64_t *out) {
    val = skip_ws(val);
    char *end = NULL;
    long long v = strtoll(val, &end, 10);
    if (end == val) return -1;
    *out = (int64_t)v;
    return 0;
}

int ub_json_get_bool(const char *val, int *out) {
    val = skip_ws(val);
    if (strncmp(val, "true", 4) == 0) {
        *out = 1;
        return 0;
    }
    if (strncmp(val, "false", 5) == 0) {
        *out = 0;
        return 0;
    }
    return -1;
}

int ub_json_is_object(const char *val) {
    val = skip_ws(val);
    return *val == '{';
}

int ub_json_is_array(const char *val) {
    val = skip_ws(val);
    return *val == '[';
}

int ub_json_is_null(const char *val) {
    val = skip_ws(val);
    return strncmp(val, "null", 4) == 0;
}

const char *ub_json_array_first(const char *arr) {
    arr = skip_ws(arr);
    if (*arr != '[') return NULL;
    const char *p = skip_ws(arr + 1);
    if (*p == ']') return NULL;
    return p;
}

const char *ub_json_array_next(const char *elem) {
    const char *p = ub_json_skip_value(elem);
    if (!p) return NULL;
    p = skip_ws(p);
    if (*p == ',') return skip_ws(p + 1);
    return NULL; /* ']' or malformed */
}
