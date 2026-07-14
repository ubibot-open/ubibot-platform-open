/* Tiny assert-based test harness. Deliberately not a full framework (Unity,
 * CMock, ...): each test file is a standalone program, so pulling in a
 * dependency for this is unnecessary weight. */
#ifndef UB_TEST_H
#define UB_TEST_H

#include <stdio.h>
#include <string.h>

static int ub_test_count = 0;
static int ub_test_failures = 0;

#define UB_CHECK(cond)                                                            \
    do {                                                                          \
        ub_test_count++;                                                         \
        if (!(cond)) {                                                           \
            ub_test_failures++;                                                  \
            fprintf(stderr, "FAIL %s:%d: %s\n", __FILE__, __LINE__, #cond);       \
        }                                                                         \
    } while (0)

#define UB_CHECK_STREQ(a, b)                                                      \
    do {                                                                         \
        ub_test_count++;                                                         \
        if (strcmp((a), (b)) != 0) {                                             \
            ub_test_failures++;                                                  \
            fprintf(stderr, "FAIL %s:%d: \"%s\" != \"%s\"\n", __FILE__, __LINE__, \
                    (a), (b));                                                   \
        }                                                                        \
    } while (0)

#define UB_CHECK_EQ_INT(a, b)                                                      \
    do {                                                                          \
        ub_test_count++;                                                         \
        long long ub_a_ = (long long)(a), ub_b_ = (long long)(b);                \
        if (ub_a_ != ub_b_) {                                                    \
            ub_test_failures++;                                                  \
            fprintf(stderr, "FAIL %s:%d: %lld != %lld\n", __FILE__, __LINE__,     \
                    ub_a_, ub_b_);                                               \
        }                                                                        \
    } while (0)

#define UB_TEST_SUMMARY()                                                                        \
    do {                                                                                        \
        fprintf(stderr, "%s: %d/%d checks passed\n", __FILE__, ub_test_count - ub_test_failures, \
                ub_test_count);                                                                  \
        return ub_test_failures ? 1 : 0;                                                        \
    } while (0)

#endif /* UB_TEST_H */
