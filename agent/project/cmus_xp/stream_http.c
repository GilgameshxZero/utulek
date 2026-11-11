/* Shared radio byte framing and bounded playlist resolution. GPL-2.0-or-later. */
#include "stream_http.h"
#include "cmus.h"
#include "keyval.h"
#include "utils.h"
#include <errno.h>
#include <stdlib.h>
#include <string.h>
#include <strings.h>
#define PLAYLIST_LIMIT (1024 * 1024)

void stream_http_close(struct stream_http_reader *r)
{
    if (!r) return;
    int saved_errno = errno;
    if (r->headers) keyvals_free(r->headers);
    free(r->url);
    r->ops->destroy(r);
    errno = saved_errno;
}

int stream_http_read(void *opaque, unsigned char *buffer, int size)
{
    struct stream_http_reader *r = opaque;
    if (size <= 0) return 0;
    if (r->metaint && !r->until_metadata) {
        unsigned char length;
        int rc = r->ops->read_raw(r, &length, 1);
        if (rc <= 0) return rc;
        unsigned bytes = (unsigned)length * 16, read = 0;
        if (bytes) {
            memset(r->metadata, 0, sizeof(r->metadata));
            while (read < bytes) {
                rc = r->ops->read_raw(r, (unsigned char *)r->metadata + read, bytes - read);
                if (rc <= 0) { errno = r->last_errno = EPROTO; return -1; }
                read += rc;
            }
        }
        r->until_metadata = r->metaint;
    }
    if (r->metaint && (unsigned)size > r->until_metadata) size = r->until_metadata;
    int rc = r->ops->read_raw(r, buffer, size);
    if (rc > 0 && r->metaint) r->until_metadata -= rc;
    return rc;
}

const char *stream_http_header(struct stream_http_reader *r, const char *name)
{
    return keyvals_get_val(r->headers, name);
}
const char *stream_http_metadata(struct stream_http_reader *r) { return r->metadata; }
int stream_http_error(struct stream_http_reader *r) { return r->last_errno; }

struct playlist_entries { char *items[16]; int count; };
static int collect_entry(void *opaque, const char *line)
{
    struct playlist_entries *entries = opaque;
    if (entries->count == 16) return 1;
    entries->items[entries->count++] = strdup(line);
    return entries->count == 16;
}

static char *join_url(const char *base, const char *relative)
{
    if (is_http_url(relative)) return strdup(relative);
    if (strchr(relative, ':') && (!strchr(relative, '/') || strchr(relative, ':') < strchr(relative, '/'))) return NULL;
    const char *scheme = strstr(base, "://");
    if (!scheme) return NULL;
    const char *path = strchr(scheme + 3, '/');
    size_t prefix;
    if (!strncmp(relative, "//", 2)) prefix = scheme - base + 1;
    else if (relative[0] == '/') prefix = path ? (size_t)(path - base) : strlen(base);
    else {
        const char *end = strpbrk(base, "?#");
        if (!end) end = base + strlen(base);
        const char *slash = path;
        for (const char *p = path; p && p < end; p++) if (*p == '/') slash = p;
        prefix = slash ? (size_t)(slash - base + 1) : strlen(base);
    }
    size_t extra_slash = !path && relative[0] != '/' && strncmp(relative, "//", 2);
    char *result = malloc(prefix + extra_slash + strlen(relative) + 1);
    if (!result) return NULL;
    memcpy(result, base, prefix);
    if (extra_slash) result[prefix++] = '/';
    strcpy(result + prefix, relative);
    return result;
}

static struct stream_http_reader *open_playlist(const char *url, atomic_int *cancel, unsigned depth, unsigned *budget)
{
    if (depth > 4 || !*budget || atomic_load(cancel)) { errno = atomic_load(cancel) ? ECANCELED : ELOOP; return NULL; }
    --*budget;
    struct stream_http_reader *r = stream_http_transport_open(url, cancel);
    if (!r) return NULL;
    if (!r->url) { stream_http_close(r); errno = ENOMEM; return NULL; }
    const char *metaint = stream_http_header(r, "icy-metaint");
    if (metaint) {
        char *end; unsigned long value = strtoul(metaint, &end, 10);
        if (*end || !value || value > 16 * 1024 * 1024) { errno = EPROTO; stream_http_close(r); return NULL; }
        r->metaint = r->until_metadata = value;
    }
    const char *type = stream_http_header(r, "content-type");
    char clean_type[128] = {0};
    if (type) {
        size_t n = strcspn(type, "; "); if (n >= sizeof(clean_type)) n = sizeof(clean_type) - 1;
        memcpy(clean_type, type, n);
    }
    int playlist = !strcasecmp(clean_type, "audio/m3u") || !strcasecmp(clean_type, "audio/x-mpegurl") ||
        !strcasecmp(clean_type, "application/vnd.apple.mpegurl") || !strcasecmp(clean_type, "application/x-mpegurl") ||
        !strcasecmp(clean_type, "audio/mpegurl") || !strcasecmp(clean_type, "audio/x-scpls");
    if (!playlist) return r;
    char *body = malloc(PLAYLIST_LIMIT + 1);
    if (!body) { stream_http_close(r); errno = ENOMEM; return NULL; }
    int used = 0, n;
    while (used < PLAYLIST_LIMIT && (n = stream_http_read(r, (unsigned char *)body + used, PLAYLIST_LIMIT - used)) > 0) used += n;
    body[used] = 0;
    if (used == PLAYLIST_LIMIT || n < 0 || strstr(body, "#EXT-X-")) {
        free(body); stream_http_close(r); errno = EPROTO; return NULL;
    }
    struct playlist_entries entries = {0};
    cmus_playlist_for_each(body, used, 0, collect_entry, &entries);
    free(body);
    char *base = strdup(r->url); stream_http_close(r);
    struct stream_http_reader *result = NULL;
    for (int i = 0; i < entries.count; i++) {
        char *candidate = base && entries.items[i] ? join_url(base, entries.items[i]) : NULL;
        if (!result && candidate && is_http_url(candidate)) result = open_playlist(candidate, cancel, depth + 1, budget);
        free(candidate); free(entries.items[i]);
    }
    free(base);
    if (!result && !atomic_load(cancel)) errno = EPROTO;
    return result;
}

struct stream_http_reader *stream_http_open(const char *url, atomic_int *cancel)
{
    unsigned budget = 32;
    return open_playlist(url, cancel, 0, &budget);
}
