/* Malformed/truncated metadata caches must not crash or read beyond records. */
#ifndef CMUS_CACHE_SOURCE
#define CMUS_CACHE_SOURCE "../cache.c"
#endif
#include CMUS_CACHE_SOURCE
#include <assert.h>

char *charset = "UTF-8";
char *clipped_text_internal = "...";
char **pl_env_vars;
int using_utf8 = 1;

static void clear_entries(void)
{
    for (int i = 0; i < HASH_SIZE; i++) {
        while (hash_table[i]) {
            struct track_info *ti = hash_table[i];
            hash_table[i] = ti->next; track_info_unref(ti);
        }
    }
    total = 0;
}

int main(int argc, char **argv)
{
    assert(argc == 2);
    cmus_config_dir = argv[1];
    assert(cache_init() == 0);
    struct cache_entry *entry = calloc(1, sizeof(*entry) + 8);
    assert(entry);
    entry->size = sizeof(*entry) + 1;
    if (valid_cache_entry(entry, entry->size)) {
        /* Old validation accepts one string, then the decoder overruns it. */
        struct track_info *ti = cache_entry_to_ti(entry);
        track_info_unref(ti);
        assert(!"accepted an entry missing codec/profile strings");
    }
    entry->size = sizeof(*entry) + 6;
    memcpy(entry->strings, "/x\0\0\0", 6);
    /* Three strings, with one extra trailing byte: deliberately malformed. */
    assert(!valid_cache_entry(entry, entry->size));
    entry->size = sizeof(*entry) + 5;
    assert(valid_cache_entry(entry, entry->size));
    size_t bytes = sizeof(cache_header) + entry->size;
    char *file_data = malloc(bytes); assert(file_data);
    memcpy(file_data, cache_header, sizeof(cache_header));
    memcpy(file_data + sizeof(cache_header), entry, entry->size);
    for (size_t length = 0; length <= bytes; length++) {
        FILE *file = fopen(cache_filename, "wb"); assert(file);
        assert(fwrite(file_data, 1, length, file) == length); fclose(file);
        int rc = read_cache();
        if (length == sizeof(cache_header) || length == bytes) assert(rc == 0);
        else assert(rc == -2);
        clear_entries();
    }
    FILE *file = fopen(cache_filename,"wb");assert(file);
    entry->size = UINT32_MAX;
    assert(fwrite(cache_header,1,sizeof(cache_header),file)==sizeof(cache_header));
    assert(fwrite(entry,1,sizeof(*entry)+5,file)==sizeof(*entry)+5);fclose(file);
    assert(read_cache()==-2);
    unlink(cache_filename);
    free(cache_filename); cache_filename = NULL;
    free(file_data);free(entry);
    puts("missing strings, every truncation boundary and oversized cache record: PASS");
    return 0;
}
