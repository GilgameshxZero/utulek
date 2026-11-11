/* Cancellable HTTP/TLS transport, driven solely by the decoder worker.
 * Bounded receive storage and a pull-style multi loop. GPL-2.0-or-later. */
#include "stream_http.h"
#include "keyval.h"
#include <curl/curl.h>
#include <pthread.h>
#include <errno.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#define RECEIVE_CAPACITY (64 * 1024)

struct curl_reader {
    struct stream_http_reader base;
    CURL *easy;
    CURLM *multi;
    struct curl_slist *request_headers, *status_aliases;
    struct growing_keyvals building;
    unsigned char bytes[RECEIVE_CAPACITY];
    size_t read_position, write_position, filled, header_bytes;
    int paused, done, final_headers, attached, owns_multi;
    long status;
    CURLcode result;
};
static pthread_once_t initialized = PTHREAD_ONCE_INIT;
static CURLcode initialization_result;
static pthread_mutex_t transfer_mutex = PTHREAD_MUTEX_INITIALIZER;
static CURLM *shared_multi;

static void cleanup_global(void)
{
    /* Normal player shutdown has joined its decoder worker. A fatal exit
     * must not race an active transfer or wait for the system DNS resolver. */
    if (pthread_mutex_trylock(&transfer_mutex)) return;
    if (shared_multi) {
        curl_multi_setopt(shared_multi, CURLMOPT_QUICK_EXIT, 1L);
        curl_multi_cleanup(shared_multi);
        shared_multi = NULL;
    }
    pthread_mutex_unlock(&transfer_mutex);
}
static void initialize(void)
{
    initialization_result = curl_global_init(CURL_GLOBAL_DEFAULT);
    if (initialization_result == CURLE_OK && atexit(cleanup_global))
        initialization_result = CURLE_FAILED_INIT;
}

static uint64_t milliseconds(void)
{
#ifdef CMUS_NATIVE_WINDOWS
    return GetTickCount64();
#else
    struct timespec now;
    clock_gettime(CLOCK_MONOTONIC, &now);
    return (uint64_t)now.tv_sec * 1000 + now.tv_nsec / 1000000;
#endif
}

static int transfer_error(struct curl_reader *r, CURLcode error)
{
    switch (error) {
    case CURLE_OPERATION_TIMEDOUT: errno = ETIMEDOUT; break;
    case CURLE_COULDNT_RESOLVE_HOST: errno = ENOENT; break;
    case CURLE_COULDNT_CONNECT: errno = ECONNREFUSED; break;
    case CURLE_TOO_MANY_REDIRECTS: errno = ELOOP; break;
    case CURLE_PEER_FAILED_VERIFICATION: case CURLE_SSL_CACERT_BADFILE: errno = EACCES; break;
    case CURLE_ABORTED_BY_CALLBACK: errno = ECANCELED; break;
    default: errno = EIO; break;
    }
    r->base.last_errno = errno;
    return -1;
}

static size_t receive_body(char *data, size_t size, size_t count, void *opaque)
{
    struct curl_reader *r = opaque;
    if (size && count > SIZE_MAX / size) return CURL_WRITEFUNC_ERROR;
    size_t length = size * count;
    if (atomic_load(r->base.cancel)) return CURL_WRITEFUNC_ERROR;
    if (length > RECEIVE_CAPACITY - r->filled) { r->paused = 1; return CURL_WRITEFUNC_PAUSE; }
    size_t first = RECEIVE_CAPACITY - r->write_position;
    if (first > length) first = length;
    memcpy(r->bytes + r->write_position, data, first);
    memcpy(r->bytes, data + first, length - first);
    r->write_position = (r->write_position + length) % RECEIVE_CAPACITY;
    r->filled += length;
    return length;
}

static size_t receive_header(char *data, size_t size, size_t count, void *opaque)
{
    struct curl_reader *r = opaque;
    if (size && count > SIZE_MAX / size) return 0;
    size_t length = size * count;
    if (length > 64 * 1024) return 0;
    if (!strncmp(data, "HTTP/", length < 5 ? length : 5) ||
            (length >= 4 && !memcmp(data, "ICY ", 4))) {
        if (r->building.keyvals) { keyvals_terminate(&r->building); keyvals_free(r->building.keyvals); }
        r->building = (struct growing_keyvals){0};
        if (r->base.headers) { keyvals_free(r->base.headers); r->base.headers = NULL; }
        r->header_bytes = 0; r->final_headers = 0;
        char line[128]; size_t n = length < sizeof(line) - 1 ? length : sizeof(line) - 1;
        memcpy(line, data, n); line[n] = 0;
        char *space = strchr(line, ' '); r->status = space ? strtol(space + 1, NULL, 10) : 0;
        return length;
    }
    if (r->final_headers) return length; /* Ignore chunked trailers. */
    r->header_bytes += length;
    if (r->header_bytes > 64 * 1024) return 0;
    if ((length == 2 && !memcmp(data, "\r\n", 2)) || (length == 1 && data[0] == '\n')) {
        if (r->status >= 200 && r->status != 301 && r->status != 302 && r->status != 303 && r->status != 307 && r->status != 308) {
            keyvals_terminate(&r->building);
            r->base.headers = r->building.keyvals;
            r->building = (struct growing_keyvals){0};
            r->final_headers = 1;
        }
        return length;
    }
    char *colon = memchr(data, ':', length);
    if (colon) {
        char *key = strndup(data, (size_t)(colon - data));
        size_t start = (size_t)(colon - data) + 1, end = length;
        while (start < end && (data[start] == ' ' || data[start] == '\t')) start++;
        while (end > start && (data[end - 1] == '\r' || data[end - 1] == '\n' || data[end - 1] == ' ')) end--;
        char *value = strndup(data + start, end - start);
        if (!key || !value) { free(key); free(value); return 0; }
        keyvals_add(&r->building, key, value); free(key);
    }
    return length;
}

static int progress(void *opaque, curl_off_t a, curl_off_t b, curl_off_t c, curl_off_t d)
{
    (void)a; (void)b; (void)c; (void)d;
    return atomic_load(((struct curl_reader *)opaque)->base.cancel) != 0;
}

static int pump(struct curl_reader *r)
{
    if (atomic_load(r->base.cancel)) { errno = r->base.last_errno = ECANCELED; return -1; }
    int running;
    CURLMcode code = curl_multi_perform(r->multi, &running);
    if (code != CURLM_OK) { errno = r->base.last_errno = EIO; return -1; }
    int remaining;
    CURLMsg *message;
    while ((message = curl_multi_info_read(r->multi, &remaining))) {
        if (message->msg == CURLMSG_DONE) { r->done = 1; r->result = message->data.result; }
    }
    return 0;
}

static void destroy_transport(struct stream_http_reader *base)
{
    struct curl_reader *r = (void *)base;
    if (r->attached) curl_multi_remove_handle(r->multi, r->easy);
    if (r->easy) curl_easy_cleanup(r->easy);
    curl_slist_free_all(r->request_headers); curl_slist_free_all(r->status_aliases);
    if (r->building.keyvals) { keyvals_terminate(&r->building); keyvals_free(r->building.keyvals); }
    if (r->owns_multi) pthread_mutex_unlock(&transfer_mutex);
    free(r);
}

static int raw_read(struct stream_http_reader *base, unsigned char *buffer, int size)
{
    struct curl_reader *r = (void *)base;
    uint64_t start = milliseconds();
    while (!r->filled) {
        if (atomic_load(base->cancel)) { errno = base->last_errno = ECANCELED; return -1; }
        if (r->done) return r->result == CURLE_OK ? 0 : transfer_error(r, r->result);
        if (r->paused) { r->paused = 0; curl_easy_pause(r->easy, CURLPAUSE_CONT); }
        if (pump(r)) return -1;
        if (r->filled || r->done) continue;
        if (milliseconds() - start >= 5000) { errno = base->last_errno = ETIMEDOUT; return -1; }
        if (curl_multi_poll(r->multi, NULL, 0, 25, NULL) != CURLM_OK) { errno = base->last_errno = EIO; return -1; }
    }
    if ((size_t)size > r->filled) size = (int)r->filled;
    size_t first = RECEIVE_CAPACITY - r->read_position;
    if (first > (size_t)size) first = size;
    memcpy(buffer, r->bytes + r->read_position, first);
    memcpy(buffer + first, r->bytes, size - first);
    r->read_position = (r->read_position + size) % RECEIVE_CAPACITY;
    r->filled -= size;
    return size;
}
static const struct stream_http_ops transport_ops = {.read_raw = raw_read, .destroy = destroy_transport};

struct stream_http_reader *stream_http_transport_open(const char *url, atomic_int *cancel)
{
    pthread_once(&initialized, initialize);
    if (initialization_result != CURLE_OK) { errno = EIO; return NULL; }
    struct curl_reader *r = calloc(1, sizeof(*r));
    if (!r) { errno = ENOMEM; return NULL; }
    r->base.ops = &transport_ops; r->base.cancel = cancel;
    /* Removing an easy handle cancels its request. Destroying its multi
     * handle would join DNS threads, blocking Stop on a stalled resolver.
     * Reuse one bounded pool; playlist candidates and player streams are
     * sequential. Refuse concurrent ownership instead of waiting forever. */
    int lock_error = pthread_mutex_trylock(&transfer_mutex);
    if (lock_error) { free(r); errno = lock_error; return NULL; }
    r->owns_multi = 1;
    if (!shared_multi) {
        shared_multi = curl_multi_init();
        if (shared_multi) {
            curl_multi_setopt(shared_multi, CURLMOPT_RESOLVE_THREADS_MAX, 2L);
            curl_multi_setopt(shared_multi, CURLMOPT_MAXCONNECTS, 4L);
            curl_multi_setopt(shared_multi, CURLMOPT_MAX_TOTAL_CONNECTIONS, 4L);
        }
    }
    r->multi = shared_multi;
    r->easy = curl_easy_init();
    r->request_headers = curl_slist_append(NULL, "Icy-MetaData: 1");
    r->status_aliases = curl_slist_append(NULL, "ICY 200 OK");
    if (!r->easy || !r->multi || !r->request_headers || !r->status_aliases) {
        errno = ENOMEM; stream_http_close(&r->base); return NULL;
    }
    curl_easy_setopt(r->easy, CURLOPT_URL, url);
    curl_easy_setopt(r->easy, CURLOPT_PROTOCOLS_STR, "http,https");
    curl_easy_setopt(r->easy, CURLOPT_REDIR_PROTOCOLS_STR, !strncmp(url, "https://", 8) ? "https" : "http,https");
    curl_easy_setopt(r->easy, CURLOPT_FOLLOWLOCATION, 1L);
    curl_easy_setopt(r->easy, CURLOPT_MAXREDIRS, 8L);
    curl_easy_setopt(r->easy, CURLOPT_CONNECTTIMEOUT_MS, 5000L);
    curl_easy_setopt(r->easy, CURLOPT_NOSIGNAL, 1L);
    curl_easy_setopt(r->easy, CURLOPT_SUPPRESS_CONNECT_HEADERS, 1L);
    curl_easy_setopt(r->easy, CURLOPT_HTTP_VERSION, CURL_HTTP_VERSION_1_1);
    curl_easy_setopt(r->easy, CURLOPT_USERAGENT, "cmus_xp/0.7");
    curl_easy_setopt(r->easy, CURLOPT_HTTPHEADER, r->request_headers);
    curl_easy_setopt(r->easy, CURLOPT_HTTP200ALIASES, r->status_aliases);
    curl_easy_setopt(r->easy, CURLOPT_WRITEFUNCTION, receive_body);
    curl_easy_setopt(r->easy, CURLOPT_WRITEDATA, r);
    curl_easy_setopt(r->easy, CURLOPT_HEADERFUNCTION, receive_header);
    curl_easy_setopt(r->easy, CURLOPT_HEADERDATA, r);
    curl_easy_setopt(r->easy, CURLOPT_XFERINFOFUNCTION, progress);
    curl_easy_setopt(r->easy, CURLOPT_XFERINFODATA, r);
    curl_easy_setopt(r->easy, CURLOPT_NOPROGRESS, 0L);
    const char *certificates = getenv("SSL_CERT_FILE");
    if (!certificates) certificates = getenv("CURL_CA_BUNDLE");
    if (certificates) curl_easy_setopt(r->easy, CURLOPT_CAINFO, certificates);
    const char *directory = getenv("SSL_CERT_DIR");
    if (directory) curl_easy_setopt(r->easy, CURLOPT_CAPATH, directory);
    if (curl_multi_add_handle(r->multi, r->easy) != CURLM_OK) { errno = EIO; stream_http_close(&r->base); return NULL; }
    r->attached = 1;
    uint64_t start = milliseconds();
    while (!r->final_headers && !r->done) {
        if (pump(r)) { stream_http_close(&r->base); return NULL; }
        if (r->final_headers || r->done) break;
        if (milliseconds() - start >= 5000) { errno = ETIMEDOUT; stream_http_close(&r->base); return NULL; }
        curl_multi_poll(r->multi, NULL, 0, 25, NULL);
    }
    if (r->done && r->result != CURLE_OK) {
        transfer_error(r, r->result); stream_http_close(&r->base); return NULL;
    }
    if (r->status != 200 || !r->base.headers) {
        errno = r->status == 404 ? ENOENT : r->status == 401 || r->status == 403 ? EACCES : EPROTO;
        stream_http_close(&r->base); return NULL;
    }
    char *effective = NULL; curl_easy_getinfo(r->easy, CURLINFO_EFFECTIVE_URL, &effective);
    r->base.url = strdup(effective ? effective : url);
    return &r->base;
}
