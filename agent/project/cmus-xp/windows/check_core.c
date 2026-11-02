/* Regression tests of the real player scaling, queue, index and window code. */
#include "../player.c"
#include "../play_queue.h"
#include <assert.h>
#include <stdio.h>

int play_library = 1, shuffle, scroll_offset, wrap_search = 1, using_utf8 = 1;
int skip_track_info, smart_artist_sort, display_artist_sort_name, search_restricted;
char *charset = "UTF-8";
static bool queued;
bool cmus_queue_active(void) { return queued; }

static void test_replaygain(void) {
    struct track_info ti = { .rg_track_gain = -6.020599913, .rg_track_peak = 0.8,
        .rg_album_gain = -12.041199827, .rg_album_peak = 0.9 };
    player_info_priv.ti = &ti;
    replaygain = RG_TRACK; replaygain_preamp = 0; replaygain_limit = 1;
    update_rg_scale(); assert(fabs(replaygain_scale - 0.5) < 1e-6);
    replaygain = RG_ALBUM; update_rg_scale(); assert(fabs(replaygain_scale - 0.25) < 1e-6);
    ti.rg_track_gain = 12; ti.rg_track_peak = 0.8;
    replaygain = RG_TRACK; update_rg_scale(); assert(fabs(replaygain_scale - 1.25) < 1e-6);
    replaygain = RG_SMART; queued = true; update_rg_scale(); assert(fabs(replaygain_scale - 1.25) < 1e-6);
    queued = false; update_rg_scale(); assert(fabs(replaygain_scale - 0.25) < 1e-6);
    ti.rg_track_gain = NAN; replaygain = RG_TRACK_PREFERRED;
    update_rg_scale(); assert(fabs(replaygain_scale - 0.25) < 1e-6);
    ti.rg_album_gain = NAN; update_rg_scale(); assert(replaygain_scale == 1.0);
    ti.rg_track_gain = INFINITY; replaygain = RG_TRACK;
    update_rg_scale(); assert(replaygain_scale == 1.0);
    ti.rg_track_gain = 1000000; ti.rg_track_peak = 1.0; replaygain_limit = 0;
    update_rg_scale(); assert(replaygain_scale == 1.0);
    soft_vol = 0; soft_vol_l = soft_vol_r = 100;
    replaygain_scale = 0.5; consumer_pos = scale_pos = 0;
    buffer_sf = sf_rate(48000) | sf_channels(2) | sf_bits(16) | sf_signed(1);
    int16_t pcm[] = {10000, -10000, 20000, -20000}; unsigned int size = sizeof(pcm);
    scale_samples((char *)pcm, &size);
    assert(pcm[0] == 5000 && pcm[1] == -5000 && pcm[2] == 10000 && pcm[3] == -10000);
    scale_samples((char *)pcm, &size); assert(pcm[0] == 5000); /* not scaled twice on retry */
    assert(scale_sample_s24le(0x7fffff, 2 * SOFT_VOL_SCALE) == 0x7fffff);
    int32_t s32 = INT32_MAX; scale_sample_int32_t(&s32, 0, 2 * SOFT_VOL_SCALE, 0); assert(s32 == INT32_MAX);
    char packed[6];
    write_s24le(packed, -1000000); write_s24le(packed + 3, 1000000);
    scale_samples_s24le(packed, sizeof(packed), SOFT_VOL_SCALE / 2, SOFT_VOL_SCALE / 2);
    assert(read_s24le(packed) == -500000 && read_s24le(packed + 3) == 500000);
    player_info_priv.ti = NULL;
    puts("ReplayGain track/album/smart/fallback/peak limit and PCM scaling: PASS");
}
static void test_queue(void) {
    play_queue_init();
    struct track_info *fifo[2];
    for (int i = 0; i < 2; i++) {
        fifo[i] = track_info_new(i ? "A.opus" : "Z.opus");
        fifo[i]->duration = 1; track_info_set_comments(fifo[i], keyvals_new(0));
        play_queue_append(fifo[i], NULL);
    }
    for (int i = 0; i < 2; i++) {
        struct track_info *got = play_queue_remove(); assert(got == fifo[i]);
        track_info_unref(got); track_info_unref(fifo[i]);
    }
    free(pq_editable.shared->sort_keys);
    pq_editable.shared->sort_keys = malloc(2 * sizeof(sort_key_t));
    pq_editable.shared->sort_keys[0] = SORT_FILENAME;
    pq_editable.shared->sort_keys[1] = SORT_INVALID;
    for (int i = 0; i < 50000; i++) {
        char name[128]; snprintf(name, sizeof(name), "C:/Music/track_%05d.opus", i);
        struct track_info *ti = track_info_new(name);
        ti->duration = 180; track_info_set_comments(ti, keyvals_new(0));
        play_queue_append(ti, NULL); track_info_unref(ti);
    }
    assert(pq_editable.nr_tracks == 50000 && play_queue_total_time() == 9000000);
    struct track_info *first = track_info_new("C:/Music/priority.opus");
    first->duration = 1; track_info_set_comments(first, keyvals_new(0));
    play_queue_prepend(first, NULL); track_info_unref(first);
    first = play_queue_remove(); assert(strstr(first->filename, "priority.opus")); track_info_unref(first);
    for (int i = 0; i < 50000; i++) {
        struct track_info *ti = play_queue_remove();
        char name[128]; snprintf(name, sizeof(name), "C:/Music/track_%05d.opus", i);
        assert(ti && !strcmp(ti->filename, name)); track_info_unref(ti);
    }
    assert(!play_queue_remove() && pq_editable.nr_tracks == 0 && !play_queue_total_time());
    window_set_nr_rows(pq_editable.shared->win, 1);
    window_set_nr_rows(pq_editable.shared->win, 100);
    window_free(pq_editable.shared->win);
    free(pq_editable.shared->sort_keys);
    searchable_free(pq_editable.shared->searchable);
    puts("50,000-track queue/index, prepend/FIFO, accounting and window rows: PASS");
}
int main(void) { test_replaygain(); test_queue(); return 0; }
