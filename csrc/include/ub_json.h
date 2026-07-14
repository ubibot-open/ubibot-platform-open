/* Minimal, allocation-free JSON reader.
 *
 * This is not a general-purpose JSON library: it only implements what a
 * device needs to read the fixed, known response shapes coming back from
 * the UbiBot API (see ub_protocol.h) — find a key's value inside an
 * object without being confused by same-named keys nested deeper, decode a
 * JSON string/number/bool, and step through an array element by element.
 * Everything works on pointers into the caller's own read buffer; nothing
 * here calls malloc. */
#ifndef UB_JSON_H
#define UB_JSON_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Finds "key" as a direct member of the object starting at `obj` (obj must
 * point at the object's opening '{'). Returns a pointer to the start of the
 * value (first non-whitespace character after the colon), or NULL if the
 * key isn't present at this object's own level (nested objects/arrays are
 * skipped over, not searched into). */
const char *ub_json_find_key(const char *obj, const char *key);

/* Value decoders. Each takes a pointer returned by ub_json_find_key (or by
 * the array iterators below) and returns 0 on success, -1 on a type/format
 * mismatch or truncation. */
int ub_json_get_string(const char *val, char *out, size_t out_cap);
int ub_json_get_int64(const char *val, int64_t *out);
int ub_json_get_bool(const char *val, int *out);

/* True if val points at a JSON object ('{') / array ('[') / null. */
int ub_json_is_object(const char *val);
int ub_json_is_array(const char *val);
int ub_json_is_null(const char *val);

/* Returns a pointer one past the value at `val` (handles strings, numbers,
 * true/false/null, and nested objects/arrays), skipping trailing
 * whitespace. Used to walk arrays and to bound a sub-object's raw text. */
const char *ub_json_skip_value(const char *val);

/* Array iteration: `arr` must point at the array's opening '['.
 * ub_json_array_first returns the first element (or NULL if empty/absent),
 * ub_json_array_next advances from an element to the next one (or NULL
 * once the array is exhausted). */
const char *ub_json_array_first(const char *arr);
const char *ub_json_array_next(const char *elem);

#ifdef __cplusplus
}
#endif

#endif /* UB_JSON_H */
