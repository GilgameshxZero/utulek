/* Deterministic checks of the production WaveOut code, with a model driver. */
#define CMUS_WAVEOUT_TEST
#include "../op/waveout.c"
#include <assert.h>

static ULONGLONG ticks, due;
static unsigned int bytes_per_second;
static int resets, paused, lost, stalled, fail_write;
static struct { WAVEHDR *hdr; ULONGLONG end; } submitted[4096];
static int first, last;
static unsigned char captured[1024 * 1024];
static size_t played;
ULONGLONG GetTickCount64(void) { return ticks; }
void Sleep(unsigned int ms) {
    ticks += ms;
    while (!paused && !lost && !stalled && first < last && submitted[first].end <= ticks) {
        WAVEHDR *h = submitted[first++].hdr;
        assert(played + h->dwBufferLength <= sizeof(captured));
        memcpy(captured + played, h->lpData, h->dwBufferLength);
        played += h->dwBufferLength;
        h->dwFlags |= WHDR_DONE;
    }
}
int waveOutOpen(HWAVEOUT *h, int d, const WAVEFORMATEX *f, int a, int b, int c) {
    (void)d; (void)a; (void)b; (void)c;
    if (lost) return MMSYSERR_NODRIVER;
    assert(f->nBlockAlign == f->nChannels * f->wBitsPerSample / 8);
    if (f->wBitsPerSample <= 16) assert(f->cbSize == 0);
    else assert(f->wFormatTag == WAVE_FORMAT_EXTENSIBLE);
    bytes_per_second = f->nAvgBytesPerSec; *h = (void *)1;
    first = last = 0; due = ticks; paused = 0; return 0;
}
int waveOutClose(HWAVEOUT h) { (void)h; assert(first == last); return 0; }
int waveOutReset(HWAVEOUT h) {
    (void)h; resets++;
    while (first < last) submitted[first++].hdr->dwFlags |= WHDR_DONE;
    due = ticks; return 0;
}
int waveOutPrepareHeader(HWAVEOUT h, WAVEHDR *w, int size) { (void)h; (void)size; w->dwFlags = WHDR_PREPARED; return 0; }
int waveOutUnprepareHeader(HWAVEOUT h, WAVEHDR *w, int size) {
    (void)h; (void)size;
    if ((w->dwFlags & WHDR_PREPARED) && !(w->dwFlags & WHDR_DONE) && !fail_write) return WAVERR_STILLPLAYING;
    w->dwFlags = 0; return 0;
}
int waveOutWrite(HWAVEOUT h, WAVEHDR *w, int size) {
    (void)h; (void)size;
    if (fail_write) return MMSYSERR_NODRIVER;
    assert(last < 4096);
    if (due < ticks) due = ticks;
    due += (ULONGLONG)w->dwBufferLength * 1000 / bytes_per_second + 1;
    submitted[last].hdr = w; submitted[last++].end = due; return 0;
}
int waveOutPause(HWAVEOUT h) { (void)h; paused = 1; return 0; }
int waveOutRestart(HWAVEOUT h) { (void)h; paused = 0; return lost ? MMSYSERR_NODRIVER : 0; }
int waveOutGetPosition(HWAVEOUT h, MMTIME *p, int size) { (void)h; (void)p; (void)size; return lost ? MMSYSERR_NODRIVER : 0; }
void _debug_print(const char *f, const char *fmt, ...) { (void)f; (void)fmt; }

static void submit(const unsigned char *data, int size) {
    while (size) {
        int rc = waveout_write((const char *)data, size);
        assert(rc >= 0);
        data += rc; size -= rc;
        if (!rc) Sleep(5);
    }
}
int main(void) {
    sample_format_t sf = sf_rate(48000) | sf_channels(2) | sf_bits(16) | sf_signed(1);
    assert(waveout_init() == 0);
    assert(waveout_open(sf, NULL) == 0);
    unsigned char track_a[997 * 4], track_b[25001 * 4];
    memset(track_a, 0x11, sizeof(track_a)); memset(track_b, 0x22, sizeof(track_b));
    submit(track_a, sizeof(track_a)); submit(track_b, sizeof(track_b));
    assert(resets == 0); assert(waveout_close() == 0);
    assert(resets == 0 && played == sizeof(track_a) + sizeof(track_b));
    assert(!memcmp(captured, track_a, sizeof(track_a)));
    assert(!memcmp(captured + sizeof(track_a), track_b, sizeof(track_b)));
    puts("gapless queue and complete track endings: PASS");
    assert(waveout_open(sf, NULL) == 0);
    submit(track_a, sizeof(track_a));
    Sleep(100);
    assert(waveout_buffer_space() == buffer_count * buffer_size);
    assert(waveout_set_buffer_count("4") == -OP_ERROR_ERRNO && errno == EBUSY);
    assert(waveout_close() == 0);
    puts("partial-ring reclaim and live-option safety: PASS");
    assert(waveout_open(sf, NULL) == 0); submit(track_a, sizeof(track_a));
    assert(waveout_pause() == 0); Sleep(20000); assert(waveout_buffer_space() >= 0);
    assert(waveout_close() == 0);
    puts("paused drain without false stall: PASS");
    assert(waveout_open(sf, NULL) == 0); submit(track_a, sizeof(track_a));
    assert(waveout_drop() == 0 && buffers_free == buffer_count);
    assert(waveout_close() == 0);
    puts("explicit stop and seek discard queued audio: PASS");
    assert(waveout_open(sf, NULL) == 0); fail_write = 1;
    assert(waveout_write((char *)track_a, sizeof(track_a)) == -OP_ERROR_ERRNO && errno == ENODEV);
    fail_write = 0; assert(waveout_close() == 0);
    assert(waveout_open(sf, NULL) == 0); submit(track_a, sizeof(track_a)); lost = 1;
    assert(waveout_buffer_space() == -OP_ERROR_ERRNO && errno == ENODEV);
    assert(waveout_close() == 0); lost = 0;
    assert(waveout_open(sf, NULL) == 0); submit(track_a, sizeof(track_a)); stalled = 1; ticks += 11000;
    assert(waveout_buffer_space() == -OP_ERROR_ERRNO && errno == ENODEV);
    assert(waveout_close() == 0); stalled = 0;
    puts("device removal, submission failure, bounded stalled drain: PASS");
    assert(waveout_open((sf & ~SF_BITS_MASK) | sf_bits(32), NULL) == 0);
    assert(waveout_close() == 0); assert(waveout_exit() == 0);
    puts("32-bit output format: PASS");
    return 0;
}
