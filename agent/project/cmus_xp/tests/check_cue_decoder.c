/* Exercise the real input layer and CUE child decoder. GPL-2.0-or-later. */
#include "../input.h"
#include "../keyval.h"
#include "../windows/static_plugins.h"
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>

char *charset = "UTF-8", *clipped_text_internal = "...";
int using_utf8 = 1;
#define DECLARE(name) \
extern const int name##_ip_priority; \
extern const struct input_plugin_ops name##_ip_ops; \
extern const struct input_plugin_opt name##_ip_options[]; \
extern const char *const name##_ip_extensions[], *const name##_ip_mime_types[]
DECLARE(wav); DECLARE(ffmpeg); DECLARE(cue);
#define INPUT(name) {#name, &name##_ip_priority, &name##_ip_ops, name##_ip_options, name##_ip_extensions, name##_ip_mime_types}
const struct cmus_static_input cmus_static_inputs[] = {INPUT(wav), INPUT(ffmpeg), INPUT(cue), {NULL}};

int main(int argc, char **argv)
{
    if (argc != 3) return 2;
    ip_load_plugins();
    struct input_plugin *input = ip_new(argv[1]);
    int rc = ip_open(input);
    if (rc) { ip_delete(input); return 3; }
    ip_setup(input);
    struct keyval *comments = NULL;
    ip_read_comments(input, &comments);
    const char *title = comments ? keyvals_get_val(comments, "title") : NULL;
    sample_format_t sf = ip_get_sf(input);
    char *codec = ip_codec(input);
    char *profile = ip_codec_profile(input);
    FILE *output = fopen(argv[2], "wb");
    if (!output) return 4;
    unsigned char buffer[8192], first[8192];
    int first_size = 0, count;
    size_t total = 0;
    while ((count = ip_read(input, (char *)buffer, sizeof(buffer))) != 0) {
        if (count == -1 && errno == EAGAIN) continue;
        if (count < 0 || count > sizeof(buffer)) return 5;
        if (!first_size) { first_size = count; memcpy(first, buffer, count); }
        if (fwrite(buffer, 1, count, output) != (size_t)count) return 6;
        total += count;
    }
    fclose(output);
    int restart = ip_seek(input, 0) == 0;
    count = ip_read(input, (char *)buffer, sizeof(buffer));
    restart = restart && count == first_size && !memcmp(first, buffer, first_size);
    size_t seek_frame = total / sf_get_frame_size(sf) / 2;
    int seek = ip_seek(input, (double)seek_frame / sf_get_rate(sf)) == 0;
    count = ip_read(input, (char *)buffer, sizeof(buffer));
    FILE *reference = fopen(argv[2], "rb");
    unsigned char expected[8192];
    if (reference && count > 0) {
        fseek(reference, seek_frame * sf_get_frame_size(sf), SEEK_SET);
        seek = seek && fread(expected, 1, count, reference) == (size_t)count && !memcmp(buffer, expected, count);
    } else seek = 0;
    if (reference) fclose(reference);
    int invalid_seek = ip_seek(input, NAN) < 0 && ip_seek(input, -1) < 0;
    printf("{\"bytes\":%zu,\"rate\":%u,\"channels\":%u,\"bits\":%u,\"duration\":%d,\"restart\":%s,\"seek\":%s,\"invalid_seek\":%s,\"title_present\":%s,\"codec_present\":%s}\n",
        total, sf_get_rate(sf), sf_get_channels(sf), sf_get_bits(sf), ip_duration(input),
        restart ? "true" : "false", seek ? "true" : "false", invalid_seek ? "true" : "false",
        title ? "true" : "false", codec ? "true" : "false");
    if (comments) keyvals_free(comments);
    ip_delete(input);
    free(codec); free(profile);
    return restart && seek && invalid_seek ? 0 : 7;
}
