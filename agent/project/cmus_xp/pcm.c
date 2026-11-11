/*
 * Copyright 2008-2013 Various Authors
 * Copyright 2004 Timo Hirvonen
 *
 * This program is free software; you can redistribute it and/or
 * modify it under the terms of the GNU General Public License as
 * published by the Free Software Foundation; either version 2 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, see <http://www.gnu.org/licenses/>.
 */

#include "pcm.h"
#include "utils.h"

#include <stdint.h>
#include <stdlib.h>

static int speaker_gain(channel_position_t position, int left, int right)
{
	switch (position) {
	case CHANNEL_POSITION_FRONT_LEFT: case CHANNEL_POSITION_REAR_LEFT:
	case CHANNEL_POSITION_SIDE_LEFT: case CHANNEL_POSITION_FRONT_LEFT_OF_CENTER:
	case CHANNEL_POSITION_TOP_FRONT_LEFT: case CHANNEL_POSITION_TOP_REAR_LEFT:
		return left;
	case CHANNEL_POSITION_FRONT_RIGHT: case CHANNEL_POSITION_REAR_RIGHT:
	case CHANNEL_POSITION_SIDE_RIGHT: case CHANNEL_POSITION_FRONT_RIGHT_OF_CENTER:
	case CHANNEL_POSITION_TOP_FRONT_RIGHT: case CHANNEL_POSITION_TOP_REAR_RIGHT:
		return right;
	default:
		return ((int64_t)left + right) / 2;
	}
}

void pcm_scale_channels(void *buffer, size_t count, sample_format_t sf,
		const channel_position_t *map, int left, int right)
{
	unsigned channels = sf_get_channels(sf), bits = sf_get_bits(sf), bytes = bits / 8;
	if (!channels || channels > CHANNELS_MAX ||
			(bits != 8 && bits != 16 && bits != 24 && bits != 32)) return;
	size_t frames = count / (bytes * channels);
	if (!left && !right) {
		size_t samples = frames * channels;
		unsigned char *pcm = buffer;
		memset(pcm, !sf_get_signed(sf) && bytes == 1 ? 128 : 0, samples * bytes);
		if (!sf_get_signed(sf) && bytes > 1) {
			unsigned high = sf_get_bigendian(sf) ? 0 : bytes - 1;
			for (size_t sample = 0; sample < samples; sample++) pcm[sample * bytes + high] = 128;
		}
		return;
	}
	int gains[CHANNELS_MAX];
	for (unsigned i = 0; i < channels; i++)
		gains[i] = speaker_gain(map && channel_map_valid(map) ? map[i] : CHANNEL_POSITION_INVALID, left, right);
	int64_t half = INT64_C(1) << (bits - 1);
	unsigned char *pcm = buffer;
	for (size_t frame = 0; frame < frames; frame++) {
		for (unsigned channel = 0; channel < channels; channel++, pcm += bytes) {
			if (gains[channel] == 65536) continue;
			uint32_t value = 0;
			for (unsigned i = 0; i < bytes; i++) {
				unsigned index = sf_get_bigendian(sf) ? bytes - 1 - i : i;
				value |= (uint32_t)pcm[index] << (i * 8);
			}
			int64_t sample = value;
			if (!sf_get_signed(sf)) sample -= half;
			else if (value & (uint32_t)half) sample -= half * 2;
			sample = sample * gains[channel] / 65536;
			if (sample < -half) sample = -half;
			if (sample >= half) sample = half - 1;
			if (!sf_get_signed(sf)) sample += half;
			value = (uint32_t)sample;
			for (unsigned i = 0; i < bytes; i++) {
				unsigned index = sf_get_bigendian(sf) ? bytes - 1 - i : i;
				pcm[index] = value >> (i * 8);
			}
		}
	}
}

/*
 * Functions to convert PCM to 16-bit signed little-endian stereo
 *
 * Conversion for 8-bit PCM):
 *   1. phase
 *      unsigned -> signed
 *      mono -> stereo
 *      8 -> 16
 *
 * Conversion for 16-bit PCM:
 *   1. phase
 *      be -> le
 *      unsigned -> signed
 *
 *   2. phase
 *      mono -> stereo
 *
 * There's no reason to split 8-bit conversion to 2 phases because we need to
 * use separate buffer for 8->16 conversion anyway.
 *
 * Conversions for 16-bit stereo can be done in place. 16-bit mono needs to be
 * converted to stereo so it's worthwhile to split the conversion to 2 phases.
 */

static void convert_u8_1ch_to_s16_2ch(void *dst, const void *src, int count)
{
	int16_t *d = dst;
	const uint8_t *s = src;
	int i, j = 0;

	for (i = 0; i < count; i++) {
		int16_t sample = ((int)s[i] - 128) * 256;
		d[j++] = sample;
		d[j++] = sample;
	}
}

static void convert_s8_1ch_to_s16_2ch(void *dst, const void *src, int count)
{
	int16_t *d = dst;
	const int8_t *s = src;
	int i, j = 0;

	for (i = 0; i < count; i++) {
		int16_t sample = (int)s[i] * 256;
		d[j++] = sample;
		d[j++] = sample;
	}
}

static void convert_u8_2ch_to_s16_2ch(void *dst, const void *src, int count)
{
	int16_t *d = dst;
	const uint8_t *s = src;
	int i;

	for (i = 0; i < count; i++) {
		int16_t sample = ((int)s[i] - 128) * 256;
		d[i] = sample;
	}
}

static void convert_s8_2ch_to_s16_2ch(void *dst, const void *src, int count)
{
	int16_t *d = dst;
	const int8_t *s = src;
	int i;

	for (i = 0; i < count; i++) {
		int16_t sample = (int)s[i] * 256;
		d[i] = sample;
	}
}

static void convert_u16_le_to_s16_le(void *buf, int count)
{
	int16_t *b = buf;
	int i;

	for (i = 0; i < count; i++) {
		int sample = (uint16_t)b[i];
		sample -= 32768;
		b[i] = sample;
	}
}

static void convert_u16_be_to_s16_le(void *buf, int count)
{
	int16_t *b = buf;
	int i;

	for (i = 0; i < count; i++) {
		uint16_t u = b[i];
		int sample;

		u = swap_uint16(u);
		sample = (int)u - 32768;
		b[i] = sample;
	}
}

static void swap_s16_byte_order(void *buf, int count)
{
	int16_t *b = buf;
	int i;

	for (i = 0; i < count; i++)
		b[i] = swap_uint16(b[i]);
}

static void convert_16_1ch_to_16_2ch(void *dst, const void *src, int count)
{
	int16_t *d = dst;
	const int16_t *s = src;
	int i, j = 0;

	for (i = 0; i < count; i++) {
		d[j++] = s[i];
		d[j++] = s[i];
	}
}

/* index is ((bits >> 2) & 4) | (is_signed << 1) | (channels - 1) */
pcm_conv_func pcm_conv[8] = {
	convert_u8_1ch_to_s16_2ch,
	convert_u8_2ch_to_s16_2ch,
	convert_s8_1ch_to_s16_2ch,
	convert_s8_2ch_to_s16_2ch,

	convert_16_1ch_to_16_2ch,
	NULL,
	convert_16_1ch_to_16_2ch,
	NULL
};

/* index is ((bits >> 2) & 4) | (is_signed << 1) | bigendian */
pcm_conv_in_place_func pcm_conv_in_place[8] = {
	NULL,
	NULL,
	NULL,
	NULL,

	convert_u16_le_to_s16_le,
	convert_u16_be_to_s16_le,

#ifdef WORDS_BIGENDIAN
	swap_s16_byte_order,
	NULL,
#else
	NULL,
	swap_s16_byte_order,
#endif
};
