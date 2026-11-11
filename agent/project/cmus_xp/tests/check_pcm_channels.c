/* Exhaustive native PCM gain/endian/saturation and speaker mapping checks. */
#include "../pcm.h"
#include <assert.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

static void put(unsigned char *p, int bytes, int big, int sign, int64_t value)
{
    uint64_t raw = sign ? (uint64_t)value : value + (INT64_C(1) << (bytes * 8 - 1));
    for (int i = 0; i < bytes; i++) p[big ? bytes - 1 - i : i] = raw >> (i * 8);
}
static int64_t get(const unsigned char *p, int bytes, int big, int sign)
{
    uint64_t value = 0;
    for (int i = 0; i < bytes; i++) value |= (uint64_t)p[big ? bytes - 1 - i : i] << (i * 8);
    int64_t half = INT64_C(1) << (bytes * 8 - 1);
    return sign ? ((value & half) ? (int64_t)value - half * 2 : (int64_t)value) : (int64_t)value - half;
}
int main(void)
{
    unsigned cases = 0;
    for (int channels = 1; channels <= CHANNELS_MAX; channels++)
    for (int bytes = 1; bytes <= 4; bytes++)
    for (int big = 0; big < 2; big++)
    for (int sign = 0; sign < 2; sign++) {
        sample_format_t sf = sf_rate(48000) | sf_channels(channels) | sf_bits(bytes * 8) | sf_bigendian(big) | sf_signed(sign);
        unsigned char source[32 * 4 * 3], data[sizeof(source)];
        CHANNEL_MAP(map);
        for (int channel = 0; channel < channels; channel++)
            map[channel] = channel % 3 == 0 ? CHANNEL_POSITION_FRONT_LEFT : channel % 3 == 1 ? CHANNEL_POSITION_REAR_RIGHT : CHANNEL_POSITION_LFE;
        int64_t half = INT64_C(1) << (bytes * 8 - 1);
        int64_t values[] = {-half, half - 1, -3};
        size_t size = channels * bytes * 3;
        for (int frame = 0; frame < 3; frame++)
            for (int channel = 0; channel < channels; channel++)
                put(source + (frame * channels + channel) * bytes, bytes, big, sign, values[frame]);
        memcpy(data, source, size);
        pcm_scale_channels(data, size, sf, map, 0, 0);
        for (int i = 0; i < channels * 3; i++) assert(get(data + i * bytes, bytes, big, sign) == 0);
        memcpy(data, source, size);
        pcm_scale_channels(data, size, sf, map, 32768, 32768);
        for (int frame = 0; frame < 3; frame++)
            for (int channel = 0; channel < channels; channel++)
                assert(get(data + (frame * channels + channel) * bytes, bytes, big, sign) == values[frame] / 2);
        memcpy(data, source, size);
        pcm_scale_channels(data, size, sf, map, 131072, 131072);
        for (int frame = 0; frame < 3; frame++) {
            int64_t expected = values[frame] * 2;
            if (expected < -half) expected = -half;
            if (expected >= half) expected = half - 1;
            for (int channel = 0; channel < channels; channel++)
                assert(get(data + (frame * channels + channel) * bytes, bytes, big, sign) == expected);
        }
        memcpy(data, source, size);
        pcm_scale_channels(data, size, sf, map, 0, 65536);
        for (int frame = 0; frame < 3; frame++)
            for (int channel = 0; channel < channels; channel++) {
                int64_t expected = channel % 3 == 0 ? 0 : channel % 3 == 1 ? values[frame] : values[frame] / 2;
                assert(get(data + (frame * channels + channel) * bytes, bytes, big, sign) == expected);
            }
        cases++;
    }
    for (int channels = 1; channels <= 18; channels++) {
        CHANNEL_MAP(map);
        channel_map_init_waveex(channels, 0, map);
        assert(channel_map_valid(map));
        int permutation[CHANNELS_MAX];
        unsigned mask = channel_map_to_waveex(channels, map, permutation);
        assert(mask);
        for (int i = 0; i < channels; i++) assert(permutation[i] == i);
        channel_map_init_waveex(channels, mask, map);
        assert(channel_map_valid(map));
    }
    CHANNEL_MAP(invalid);
    channel_map_init_waveex(2, 0xffffffffU, invalid);
    assert(!channel_map_valid(invalid) && !channel_map_to_waveex(2, invalid, NULL));
    channel_map_init_waveex(4, 0, invalid);
    assert(invalid[2] == CHANNEL_POSITION_REAR_LEFT && invalid[3] == CHANNEL_POSITION_REAR_RIGHT);
    puts("WAVE speaker masks, default layouts and invalid-map rejection: PASS");
    printf("native PCM mute/gain/saturation/balance cases: %u PASS\n", cases);
    return 0;
}
