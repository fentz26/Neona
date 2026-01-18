/* ==============================================================================
 * MODULE: SIMD-Accelerated Memory Store
 * ==============================================================================
 * @context: High-performance memory map using hardware CRC32 and AVX2.
 * @goal: Minimize lookup latency and insertion time for memory items.
 * ==============================================================================
 */

#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <immintrin.h>
#include <nmmintrin.h>

#define CACHE_LINE 64
#define ALIGNED(x) __attribute__((aligned(x)))
#define RESTRICT __restrict__
#define FORCE_INLINE __attribute__((always_inline)) inline
#define LIKELY(x) __builtin_expect(!!(x), 1)
#define UNLIKELY(x) __builtin_expect(!!(x), 0)

#define LUT_CAPACITY (1 << 18) 
#define LUT_MASK (LUT_CAPACITY - 1)
#define EMPTY_SLOT 0

typedef struct ALIGNED(64) {
    uint32_t hashes[8];     // 32 bytes
    uint32_t indices[8];    // 32 bytes
} Bucket;

typedef struct {
    uint32_t* ALIGNED(64) id_offs;
    uint32_t* ALIGNED(64) content_offs;
    uint32_t* ALIGNED(64) task_id_offs;
    uint32_t* ALIGNED(64) tags_offs;
    Bucket*   ALIGNED(64) buckets;
    char*     ALIGNED(64) arena;
    uint32_t  count;
    uint32_t  capacity;
    uint32_t  arena_size;
    uint32_t  arena_used;
} MemoryStore;

// --- Hardware CRC32 Hashing ---
/* @logic: Uses SSE4.2 instructions for high-speed string hashing. */
FORCE_INLINE uint32_t _hash_fast(const char* str, uint32_t len) {
    uint64_t h = 0x12345678;
    const uint64_t* p = (const uint64_t*)str;
    while (len >= 8) {
        h = _mm_crc32_u64(h, *p++);
        len -= 8;
    }
    const uint8_t* p2 = (const uint8_t*)p;
    if (len & 4) { h = _mm_crc32_u32(h, *(const uint32_t*)p2); p2 += 4; }
    if (len & 2) { h = _mm_crc32_u16(h, *(const uint16_t*)p2); p2 += 2; }
    if (len & 1) { h = _mm_crc32_u8(h, *p2); }
    return (uint32_t)h;
}

// --- Fast Path String Comparison ---
FORCE_INLINE int _fast_streq(const char* RESTRICT s1, const char* RESTRICT s2, uint32_t len) {
    if (len >= 8) {
        if (*(const uint64_t*)s1 != *(const uint64_t*)s2) return 0;
    }
    return strcmp(s1, s2) == 0;
}

MemoryStore* memory_store_init(uint32_t initial_cap) {
    MemoryStore* s = (MemoryStore*)malloc(sizeof(MemoryStore));
    if (!s) return NULL;
    
    s->capacity = initial_cap;
    s->id_offs = (uint32_t*)aligned_alloc(64, sizeof(uint32_t) * s->capacity);
    s->content_offs = (uint32_t*)aligned_alloc(64, sizeof(uint32_t) * s->capacity);
    s->task_id_offs = (uint32_t*)aligned_alloc(64, sizeof(uint32_t) * s->capacity);
    s->tags_offs = (uint32_t*)aligned_alloc(64, sizeof(uint32_t) * s->capacity);
    
    s->buckets = (Bucket*)aligned_alloc(64, sizeof(Bucket) * LUT_CAPACITY);
    memset(s->buckets, 0, sizeof(Bucket) * LUT_CAPACITY);
    
    s->arena_size = 1024 * 1024;
    s->arena = (char*)aligned_alloc(64, s->arena_size);
    s->arena[0] = '\0';
    s->arena_used = 1;
    s->count = 0;
    
    return s;
}

static inline uint32_t _push_to_arena(MemoryStore* s, const char* str, uint32_t len) {
    if (__builtin_expect(!str || len == 0, 0)) return 0;
    uint32_t required = len + 1;
    if (__builtin_expect(s->arena_used + required > s->arena_size, 0)) {
        s->arena_size = s->arena_size * 2 + required;
        s->arena = (char*)realloc(s->arena, s->arena_size);
    }
    uint32_t off = s->arena_used;
    __builtin_memcpy(s->arena + off, str, len);
    s->arena[off + len] = '\0';
    s->arena_used += required;
    return off;
}

void memory_store_add(MemoryStore* RESTRICT s, 
                     const char* id, uint32_t id_len,
                     const char* task_id, uint32_t task_id_len,
                     const char* content, uint32_t content_len,
                     const char* tags, uint32_t tags_len) {
    
    uint32_t hash = _hash_fast(id, id_len);
    if (UNLIKELY(hash == 0)) hash = 1;
    
    // Safety: Ensure capacity before adding
    if (UNLIKELY(s->count >= s->capacity)) {
        uint32_t new_cap = s->capacity * 2;
        s->id_offs = (uint32_t*)realloc(s->id_offs, sizeof(uint32_t) * new_cap);
        s->content_offs = (uint32_t*)realloc(s->content_offs, sizeof(uint32_t) * new_cap);
        s->task_id_offs = (uint32_t*)realloc(s->task_id_offs, sizeof(uint32_t) * new_cap);
        s->tags_offs = (uint32_t*)realloc(s->tags_offs, sizeof(uint32_t) * new_cap);
        s->capacity = new_cap;
    }
    
    uint32_t idx = s->count++;
    
    s->id_offs[idx] = _push_to_arena(s, id, id_len);
    s->task_id_offs[idx] = _push_to_arena(s, task_id, task_id_len);
    s->content_offs[idx] = _push_to_arena(s, content, content_len);
    s->tags_offs[idx] = _push_to_arena(s, tags, tags_len);

    uint32_t b_idx = hash & LUT_MASK;
    while (1) {
        Bucket* b = &s->buckets[b_idx];
        __m256i chunk = _mm256_load_si256((const __m256i*)b->hashes);
        int empty_mask = _mm256_movemask_ps(_mm256_castsi256_ps(_mm256_cmpeq_epi32(chunk, _mm256_setzero_si256())));
        
        if (LIKELY(empty_mask != 0)) {
            int slot = __builtin_ctz(empty_mask);
            b->hashes[slot] = hash;
            b->indices[slot] = idx;
            return;
        }
        b_idx = (b_idx + 1) & LUT_MASK;
    }
}

/* @logic: BULK ADD. Performs a single operation for multiple items to reduce overhead. */
void memory_store_bulk_add(MemoryStore* RESTRICT s,
                          const char** ids, uint32_t* id_lens,
                          const char** task_ids, uint32_t* task_id_lens,
                          const char** contents, uint32_t* content_lens,
                          const char** tags, uint32_t* tags_lens,
                          uint32_t count) {
    for (uint32_t i = 0; i < count; i++) {
        memory_store_add(s, 
                        ids[i], id_lens[i],
                        task_ids[i], task_id_lens[i],
                        contents[i], content_lens[i],
                        tags[i], tags_lens[i]);
    }
}

int32_t memory_store_find(const MemoryStore* RESTRICT s, const char* id, uint32_t id_len) {
    uint32_t hash = _hash_fast(id, id_len);
    if (UNLIKELY(hash == 0)) hash = 1;
    
    __m256i target = _mm256_set1_epi32(hash);
    uint32_t b_idx = hash & LUT_MASK;
    
    while (1) {
        const Bucket* b = &s->buckets[b_idx];
        __m256i chunk = _mm256_load_si256((const __m256i*)b->hashes);
        __m256i cmp = _mm256_cmpeq_epi32(chunk, target);
        int mask = _mm256_movemask_ps(_mm256_castsi256_ps(cmp));
        
        if (__builtin_expect(mask != 0, 0)) {
            while (mask) {
                int i = __builtin_ctz(mask);
                uint32_t idx = b->indices[i];
                // Prefetch string data into L1 while preparing comparison
                const char* target_ptr = s->arena + s->id_offs[idx];
                __builtin_prefetch(target_ptr, 0, 3);
                if (_fast_streq(target_ptr, id, id_len)) return (int32_t)idx;
                mask &= (mask - 1);
            }
        }
        int empty_mask = _mm256_movemask_ps(_mm256_castsi256_ps(_mm256_cmpeq_epi32(chunk, _mm256_setzero_si256())));
        if (empty_mask != 0) break;
        b_idx = (b_idx + 1) & LUT_MASK;
    }
    return -1;
}

const char* memory_get_id(const MemoryStore* s, uint32_t idx)      { return s->arena + s->id_offs[idx]; }
const char* memory_get_task_id(const MemoryStore* s, uint32_t idx) { return s->arena + s->task_id_offs[idx]; }
const char* memory_get_content(const MemoryStore* s, uint32_t idx) { return s->arena + s->content_offs[idx]; }
const char* memory_get_tags(const MemoryStore* s, uint32_t idx)    { return s->arena + s->tags_offs[idx]; }
uint32_t memory_get_count(const MemoryStore* s) { return s->count; }

void memory_store_free(MemoryStore* s) {
    if (!s) return;
    free(s->id_offs); free(s->content_offs); free(s->task_id_offs); free(s->tags_offs);
    free(s->buckets); free(s->arena); free(s);
}
