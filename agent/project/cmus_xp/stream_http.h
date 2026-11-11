/* Shared cancellable radio transport interface. GPL-2.0-or-later. */
#ifndef CMUS_STREAM_HTTP_H
#define CMUS_STREAM_HTTP_H
#include <stdatomic.h>
#include <stddef.h>
struct keyval;
struct stream_http_reader;
struct stream_http_ops {
    int (*read_raw)(struct stream_http_reader *, unsigned char *, int);
    void (*destroy)(struct stream_http_reader *);
};
struct stream_http_reader {
    const struct stream_http_ops *ops;
    atomic_int *cancel;
    struct keyval *headers;
    char *url;
    int last_errno;
    unsigned metaint, until_metadata;
    char metadata[255 * 16 + 1];
};
/* Exactly one native backend implements this factory in each build. */
struct stream_http_reader *stream_http_transport_open(const char *, atomic_int *);
struct stream_http_reader *stream_http_open(const char *, atomic_int *);
int stream_http_read(void *, unsigned char *, int);
void stream_http_close(struct stream_http_reader *);
const char *stream_http_header(struct stream_http_reader *, const char *);
const char *stream_http_metadata(struct stream_http_reader *);
int stream_http_error(struct stream_http_reader *);
#endif
