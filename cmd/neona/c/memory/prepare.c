/* ==============================================================================
 * MODULE: SIMD-Accelerated JSON Parser
 * ==============================================================================
 * @context: High-performance JSON scanner for memory items.
 * @logic: 
 *   1. Use AVX2 to find structural characters (", {, }, :) 32 bytes at a time.
 *   2. Handle escaped characters properly to avoid parsing errors.
 *   3. Zero-copy pointer injection into the MemoryStore.
 * ==============================================================================
 */

#include <stdint.h>
#include <string.h>
#include <immintrin.h>
#include "map.c"

#define FORCE_INLINE __attribute__((always_inline)) inline

/* 
 * @logic: Finds delimiters such as braces, colons, and quotes using AVX2.
 * Returns a bitmask where 1 indicates a structural character match.
 */
FORCE_INLINE uint32_t _simd_find_delimiters(const char* p) {
    __m256i data = _mm256_loadu_si256((const __m256i*)p);
    
    __m256i v_quote = _mm256_set1_epi8('"');
    __m256i v_colon = _mm256_set1_epi8(':');
    __m256i v_brace_open  = _mm256_set1_epi8('{');
    __m256i v_brace_close = _mm256_set1_epi8('}');
    __m256i v_comma = _mm256_set1_epi8(',');

    __m256i cmp = _mm256_or_si256(
        _mm256_or_si256(_mm256_cmpeq_epi8(data, v_quote), _mm256_cmpeq_epi8(data, v_colon)),
        _mm256_or_si256(
            _mm256_or_si256(_mm256_cmpeq_epi8(data, v_brace_open), _mm256_cmpeq_epi8(data, v_brace_close)),
            _mm256_cmpeq_epi8(data, v_comma)
        )
    );

    return (uint32_t)_mm256_movemask_epi8(cmp);
}

/* @logic: Skip irrelevant characters using SIMD. */
static inline const char* _internal_skip_to_next(const char* p, const char* end) {
    while (p + 32 <= end) {
        uint32_t mask = _simd_find_delimiters(p);
        if (mask != 0) return p + __builtin_ctz(mask);
        p += 32;
    }
    while (p < end && *p != '"' && *p != '{' && *p != '}' && *p != ':' && *p != ',') p++;
    return p;
}

/* @logic: Find closing quote while handling escaped characters. */
static inline const char* _find_string_end(const char* p, const char* end) {
    while (p < end) {
        // SIMD scan for quote or backslash
        if (p + 32 <= end) {
            __m256i data = _mm256_loadu_si256((const __m256i*)p);
            __m256i v_quote = _mm256_set1_epi8('"');
            __m256i v_backslash = _mm256_set1_epi8('\\');
            __m256i cmp = _mm256_or_si256(
                _mm256_cmpeq_epi8(data, v_quote),
                _mm256_cmpeq_epi8(data, v_backslash)
            );
            uint32_t mask = _mm256_movemask_epi8(cmp);
            if (mask != 0) {
                int pos = __builtin_ctz(mask);
                p += pos;
                if (*p == '"') return p;
                if (*p == '\\') p += 2; // Skip escaped char
                continue;
            }
            p += 32;
        } else {
            if (*p == '"') return p;
            if (*p == '\\' && p + 1 < end) p += 2;
            else p++;
        }
    }
    return p;
}

/* @logic: Core JSON parsing logic using SIMD-based structural indexing. */
void memory_store_parse_json(MemoryStore* RESTRICT s, const char* json_raw, uint32_t len) {
    const char* p = json_raw;
    const char* end_buf = json_raw + len;

    p = _internal_skip_to_next(p, end_buf);
    if (p >= end_buf || *p != '[') {
        while(p < end_buf && *p != '[') p++;
        if(p >= end_buf) return;
    }
    p++;

    while (p < end_buf) {
        p = _internal_skip_to_next(p, end_buf);
        if (p >= end_buf || *p == ']') break;
        
        if (*p == '{') {
            p++;
            const char *id = "", *task_id = "", *content = "", *tags = "";
            uint32_t id_l = 0, tid_l = 0, cont_l = 0, tags_l = 0;

            while (p < end_buf && *p != '}') {
                p = _internal_skip_to_next(p, end_buf);
                if (p >= end_buf || *p != '"') { p++; continue; }
                
                // Parse key
                const char* key = ++p;
                p = _find_string_end(p, end_buf);
                uint32_t key_l = (uint32_t)(p - key);
                if (p < end_buf) p++; // skip closing "

                p = _internal_skip_to_next(p, end_buf); 
                if (p < end_buf && *p == ':') p++;
                
                p = _internal_skip_to_next(p, end_buf);
                if (p < end_buf && *p == '"') {
                    const char* val = ++p;
                    p = _find_string_end(p, end_buf);
                    uint32_t val_l = (uint32_t)(p - val);
                    
                    // Match keys using fingerprints
                    if (key_l == 2 && key[0] == 'i') { id = val; id_l = val_l; }
                    else if (key_l == 7 && key[1] == 'a') { task_id = val; tid_l = val_l; }
                    else if (key_l == 7 && key[1] == 'o') { content = val; cont_l = val_l; }
                    else if (key_l == 4 && key[0] == 't') { tags = val; tags_l = val_l; }
                    if (p < end_buf) p++; // skip closing "
                }
                p = _internal_skip_to_next(p, end_buf);
                if (p < end_buf && *p == ',') p++;
            }
            memory_store_add(s, id, id_l, task_id, tid_l, content, cont_l, tags, tags_l);
        }
        if (p < end_buf && *p == '}') p++;
        p = _internal_skip_to_next(p, end_buf);
        if (p < end_buf && *p == ',') p++;
    }
}
