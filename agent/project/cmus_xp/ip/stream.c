/* A bounded asynchronous radio decoder using the shared FFmpeg plugin.
 * Only this worker owns HTTP/decoder state. Player reads never wait on a server.
 * GPL-2.0-or-later. */
#include "../ip.h"
#include "../keyval.h"
#include "../convert.h"
#include "../stream_http.h"
#include <pthread.h>
#include <stdatomic.h>
#include <errno.h>
#include <stdlib.h>
#include <string.h>

extern const struct input_plugin_ops ffmpeg_ip_ops;
extern char *icecast_default_charset;

#define PCM_CAPACITY (1024 * 1024)
#define STREAM_FORMAT (sf_rate(48000) | sf_bits(32) | sf_channels(2) | sf_signed(1) | sf_host_endian())
#define META_SIZE (255 * 16 + 1)

struct stream_private {
    pthread_t worker;
    pthread_mutex_t mutex;
    pthread_cond_t space;
    atomic_int cancel;
    unsigned char *pcm;
    size_t read_position, write_position, filled;
    int done, error, error_errno;
    char *url, *charset;
    struct keyval *comments;
    char metadata[META_SIZE];
    unsigned metadata_version, published_version;
    sample_format_t sf;
    channel_position_t channel_map[CHANNELS_MAX];
    int format_ready;
};

static void publish_metadata(struct stream_private *s, struct input_plugin_data *decoder,
                             struct stream_http_reader *reader, int first)
{
    struct keyval *comments = NULL;
    if (first) {
        ffmpeg_ip_ops.read_comments(decoder, &comments);
        GROWING_KEYVALS(all);
        if (comments) { keyvals_init(&all, comments); keyvals_free(comments); }
        const char *headers[] = {"icy-name", "icy-genre", "icy-url", "icy-br"};
        const char *tags[] = {"title", "genre", "comment", "bitrate"};
        for (int i = 0; i < 4; i++) {
            const char *value = stream_http_header(reader, headers[i]);
            if (value && !keyvals_get_val_growing(&all, tags[i])) keyvals_add(&all, tags[i], to_utf8(value, s->charset));
        }
        keyvals_terminate(&all); comments = all.keyvals;
    }
    const char *metadata = stream_http_metadata(reader);
    pthread_mutex_lock(&s->mutex);
    if (first) {
        s->sf = decoder->sf;
        channel_map_copy(s->channel_map, decoder->channel_map);
        s->format_ready = 1;
    }
    if (first) { if (s->comments) keyvals_free(s->comments); s->comments = comments; s->metadata_version++; }
    if (strcmp(metadata, s->metadata)) {
        memcpy(s->metadata, metadata, META_SIZE);
        s->metadata_version++;
    }
    pthread_mutex_unlock(&s->mutex);
}

static void *decode_worker(void *opaque)
{
    struct stream_private *s = opaque;
    struct stream_http_reader *reader = stream_http_open(s->url, &s->cancel);
    int error = -IP_ERROR_ERRNO, saved_errno = errno;
    struct input_plugin_data decoder = {.filename = s->url, .fd = -1, .remote = 1,
        .stream_read = stream_http_read, .stream_opaque = reader};
    if (!reader) goto done;
    error = ffmpeg_ip_ops.open(&decoder);
    if (error) { saved_errno = stream_http_error(reader) ? stream_http_error(reader) : errno; goto done; }
    publish_metadata(s, &decoder, reader, 1);
    unsigned char buffer[64 * 1024];
    while (!atomic_load(&s->cancel)) {
        int count = ffmpeg_ip_ops.read(&decoder, (char *)buffer, sizeof(buffer));
        if (count <= 0) {
            error = count;
            saved_errno = stream_http_error(reader) ? stream_http_error(reader) : errno;
            if (stream_http_error(reader) && count < 0) error = -IP_ERROR_ERRNO;
            break;
        }
        publish_metadata(s, &decoder, reader, 0);
        size_t position = 0;
        pthread_mutex_lock(&s->mutex);
        while (position < (size_t)count && !atomic_load(&s->cancel)) {
            while (s->filled == PCM_CAPACITY && !atomic_load(&s->cancel)) pthread_cond_wait(&s->space, &s->mutex);
            size_t copy = PCM_CAPACITY - s->write_position;
            if (copy > PCM_CAPACITY - s->filled) copy = PCM_CAPACITY - s->filled;
            if (copy > (size_t)count - position) copy = (size_t)count - position;
            memcpy(s->pcm + s->write_position, buffer + position, copy);
            s->write_position = (s->write_position + copy) % PCM_CAPACITY;
            s->filled += copy; position += copy;
        }
        pthread_mutex_unlock(&s->mutex);
    }
    ffmpeg_ip_ops.close(&decoder);
done:
    stream_http_close(reader);
    pthread_mutex_lock(&s->mutex);
    s->done = 1; s->error = error; s->error_errno = saved_errno;
    pthread_mutex_unlock(&s->mutex);
    return NULL;
}

static int stream_open(struct input_plugin_data *data)
{
    struct stream_private *s = calloc(1, sizeof(*s));
    if (!s) return -IP_ERROR_ERRNO;
    s->pcm = malloc(PCM_CAPACITY);
    s->url = strdup(data->filename);
    s->charset = strdup(icecast_default_charset ? icecast_default_charset : "UTF-8");
    data->metadata = calloc(META_SIZE, 1);
    if (!s->pcm || !s->url || !s->charset || !data->metadata) {
        free(s->pcm); free(s->url); free(s->charset); free(data->metadata); data->metadata = NULL; free(s);
        errno = ENOMEM; return -IP_ERROR_ERRNO;
    }
    atomic_init(&s->cancel, 0);
    int error = pthread_mutex_init(&s->mutex, NULL);
    if (!error) {
        error = pthread_cond_init(&s->space, NULL);
        if (error) pthread_mutex_destroy(&s->mutex);
        else {
            error = pthread_create(&s->worker, NULL, decode_worker, s);
            if (error) { pthread_cond_destroy(&s->space); pthread_mutex_destroy(&s->mutex); }
        }
    }
    if (error) {
        free(s->pcm); free(s->url); free(s->charset); free(data->metadata); data->metadata = NULL; free(s);
        errno = error; return -IP_ERROR_ERRNO;
    }
    data->private = s;
    data->sf = STREAM_FORMAT;
    channel_map_init_stereo(data->channel_map);
    atomic_store(&data->format_state, IP_FORMAT_PENDING);
    return 0;
}

static int stream_close(struct input_plugin_data *data)
{
    struct stream_private *s = data->private;
    atomic_store(&s->cancel, 1);
    pthread_mutex_lock(&s->mutex); pthread_cond_broadcast(&s->space); pthread_mutex_unlock(&s->mutex);
    pthread_join(s->worker, NULL);
    pthread_cond_destroy(&s->space); pthread_mutex_destroy(&s->mutex);
    if (s->comments) keyvals_free(s->comments);
    free(s->pcm); free(s->url); free(s->charset); free(s);
    data->private = NULL;
    return 0;
}

static int stream_read(struct input_plugin_data *data, char *buffer, int count)
{
    struct stream_private *s = data->private;
    pthread_mutex_lock(&s->mutex);
    if (atomic_load(&data->format_state) == IP_FORMAT_PENDING && s->format_ready) {
        data->sf = s->sf;
        channel_map_copy(data->channel_map, s->channel_map);
        atomic_store(&data->format_state, IP_FORMAT_CHANGED);
        pthread_mutex_unlock(&s->mutex);
        errno = EAGAIN;
        return -1;
    }
    if (s->metadata_version != s->published_version) {
        memcpy(data->metadata, s->metadata, META_SIZE);
        data->metadata_changed = 1;
        s->published_version = s->metadata_version;
    }
    if (!s->filled) {
        int result = s->done ? s->error : -1;
        errno = s->done ? s->error_errno : EAGAIN;
        pthread_mutex_unlock(&s->mutex);
        return result;
    }
    if ((size_t)count > s->filled) count = (int)s->filled;
    count -= count % sf_get_frame_size(data->sf);
    if (!count) {
        pthread_mutex_unlock(&s->mutex);
        errno = EAGAIN;
        return -1;
    }
    size_t first = PCM_CAPACITY - s->read_position;
    if (first > (size_t)count) first = count;
    memcpy(buffer, s->pcm + s->read_position, first);
    memcpy(buffer + first, s->pcm, count - first);
    s->read_position = (s->read_position + count) % PCM_CAPACITY;
    s->filled -= count;
    pthread_cond_signal(&s->space);
    pthread_mutex_unlock(&s->mutex);
    return count;
}

static int stream_comments(struct input_plugin_data *data, struct keyval **comments)
{
    struct stream_private *s = data->private;
    pthread_mutex_lock(&s->mutex);
    *comments = s->comments ? keyvals_dup(s->comments) : keyvals_new(0);
    pthread_mutex_unlock(&s->mutex);
    return 0;
}
static int stream_seek(struct input_plugin_data *data, double offset) { (void)data; (void)offset; return -IP_ERROR_FUNCTION_NOT_SUPPORTED; }
static int stream_duration(struct input_plugin_data *data) { (void)data; return -IP_ERROR_FUNCTION_NOT_SUPPORTED; }
static long stream_bitrate(struct input_plugin_data *data) { (void)data; return -IP_ERROR_FUNCTION_NOT_SUPPORTED; }
static char *stream_codec(struct input_plugin_data *data) { (void)data; return strdup("radio"); }
static char *stream_profile(struct input_plugin_data *data) { (void)data; return NULL; }

const struct input_plugin_ops ip_ops = {
    .open = stream_open, .close = stream_close, .read = stream_read, .seek = stream_seek,
    .read_comments = stream_comments, .duration = stream_duration, .bitrate = stream_bitrate,
    .bitrate_current = stream_bitrate, .codec = stream_codec, .codec_profile = stream_profile
};
const int ip_priority = 60;
const char * const ip_extensions[] = {NULL};
const char * const ip_mime_types[] = {"application/x-cmus-stream", NULL};
const struct input_plugin_opt ip_options[] = {{NULL}};
const unsigned ip_abi_version = IP_ABI_VERSION;
