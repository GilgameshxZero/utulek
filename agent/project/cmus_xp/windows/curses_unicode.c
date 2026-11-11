/* Preserve Unicode scalars, combining clusters and display-cell widths in the
 * PDCurses screen model. No terminal writes happen outside curses' renderer.
 * GPL-2.0-or-later. */
#include "curses_unicode.h"
#include "../uchar.h"
#include "../xmalloc.h"
#include <string.h>
#include <stdint.h>

#define CLUSTER_BASE 0x110000
#define CLUSTER_BUCKETS 1024
struct cluster {
    struct cluster *next;
    unsigned int code;
    int bytes, length;
    WCHAR *wide;
    char text[];
};
static struct cluster *buckets[CLUSTER_BUCKETS];
static struct cluster **clusters;
static unsigned int cluster_count, cluster_capacity;

static unsigned int intern_cluster(const char *str, int bytes)
{
    uint32_t hash = 2166136261u;
    for (int i = 0; i < bytes; i++) hash = (hash ^ (unsigned char)str[i]) * 16777619u;
    unsigned int bucket = hash % CLUSTER_BUCKETS;
    for (struct cluster *p = buckets[bucket]; p; p = p->next)
        if (p->bytes == bytes && !memcmp(p->text, str, bytes)) return p->code;
    if (CLUSTER_BASE + cluster_count >= CMUS_WIDE_CONTINUATION) return 0xfffd;
    struct cluster *p = xmalloc(sizeof(*p) + bytes);
    p->bytes = bytes;
    memcpy(p->text, str, bytes);
    p->length = MultiByteToWideChar(CP_UTF8, 0, str, bytes, NULL, 0);
    p->wide = xnew(WCHAR, p->length);
    MultiByteToWideChar(CP_UTF8, 0, str, bytes, p->wide, p->length);
    /* Legacy ConPTY/conhost stores decomposed accents as extra console cells.
     * Canonical composition preserves the glyph while keeping its width. */
    int normalized_length = NormalizeString(NormalizationC, p->wide, p->length, NULL, 0);
    if (normalized_length > 0) {
        WCHAR *normalized = xnew(WCHAR, normalized_length);
        int count = NormalizeString(NormalizationC, p->wide, p->length,
                                    normalized, normalized_length);
        if (count > 0) {
            free(p->wide);
            p->wide = normalized;
            p->length = count;
        } else free(normalized);
    }
    p->code = CLUSTER_BASE + cluster_count;
    p->next = buckets[bucket];
    buckets[bucket] = p;
    if (cluster_count == cluster_capacity) {
        cluster_capacity = cluster_capacity * 2 + 32;
        clusters = xrealloc(clusters, cluster_capacity * sizeof(*clusters));
    }
    clusters[cluster_count++] = p;
    return p->code;
}

const WCHAR *win_curses_cluster(unsigned int code, int *length)
{
    if (code < CLUSTER_BASE || code - CLUSTER_BASE >= cluster_count) return NULL;
    struct cluster *p = clusters[code - CLUSTER_BASE];
    *length = p->length;
    return p->wide;
}

int win_curses_waddnstr(WINDOW *win, const char *str, int n)
{
    if (!win || !str) return ERR;
    char *bounded = n < 0 ? NULL : xstrndup(str, n);
    if (bounded) str = bounded;
    int i = 0, result = OK;
    while (str[i]) {
        int start = i;
        uchar code = u_get_char(str, &i);
        if (code & U_INVALID_MASK) code = 0xfffd;
        int width = u_char_width(code);
        if (!width) continue;
        int end = i;
        while (str[end]) {
            int next = end;
            uchar mark = u_get_char(str, &next);
            if (u_char_width(mark) != 0) break;
            end = next;
        }
        if (end != i) code = intern_cluster(str + start, end - start);
        i = end;
        if (width == 2 && win->_curx + 1 >= win->_maxx) {
            result = waddch(win, ' ');
        } else {
            result = waddch(win, code);
            if (result == OK && width == 2)
                result = waddch(win, CMUS_WIDE_CONTINUATION);
        }
        if (result == ERR) break;
    }
    free(bounded);
    return result;
}
