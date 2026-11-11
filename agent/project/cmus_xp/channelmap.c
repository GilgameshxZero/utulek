/*
 * Copyright 2011-2013 Various Authors
 * Copyright 2011 Johannes Weißl
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

#include "channelmap.h"
#include "utils.h"

static const channel_position_t channel_map_waveex[] = {
		CHANNEL_POSITION_FRONT_LEFT,
		CHANNEL_POSITION_FRONT_RIGHT,
		CHANNEL_POSITION_FRONT_CENTER,
		CHANNEL_POSITION_LFE,
		CHANNEL_POSITION_REAR_LEFT,
		CHANNEL_POSITION_REAR_RIGHT,
		CHANNEL_POSITION_FRONT_LEFT_OF_CENTER,
		CHANNEL_POSITION_FRONT_RIGHT_OF_CENTER,
		CHANNEL_POSITION_REAR_CENTER,
		CHANNEL_POSITION_SIDE_LEFT,
		CHANNEL_POSITION_SIDE_RIGHT,
		CHANNEL_POSITION_TOP_CENTER,
		CHANNEL_POSITION_TOP_FRONT_LEFT,
		CHANNEL_POSITION_TOP_FRONT_CENTER,
		CHANNEL_POSITION_TOP_FRONT_RIGHT,
		CHANNEL_POSITION_TOP_REAR_LEFT,
		CHANNEL_POSITION_TOP_REAR_CENTER,
		CHANNEL_POSITION_TOP_REAR_RIGHT
	};

void channel_map_init_waveex(int channels, unsigned int mask, channel_position_t *map)
{
	for (int i = 0; i < CHANNELS_MAX; i++) map[i] = CHANNEL_POSITION_INVALID;
	if (channels < 1 || channels > N_ELEMENTS(channel_map_waveex)) return;
	if (channels == 1 && (!mask || mask == 4)) {
		map[0] = CHANNEL_POSITION_MONO;
		return;
	}
	if (!mask) {
		static const unsigned defaults[] = {0, 4, 3, 7, 0x33, 0x37, 0x3f, 0x70f, 0x63f};
		mask = channels < N_ELEMENTS(defaults) ? defaults[channels] : (1U << channels) - 1;
	}
	unsigned known = (1U << N_ELEMENTS(channel_map_waveex)) - 1;
	if (mask & ~known) return;
	unsigned count = 0;
	for (unsigned bits = mask; bits; bits &= bits - 1) count++;
	if (count != (unsigned)channels) return;
	int j = 0;
	for (unsigned i = 0; i < N_ELEMENTS(channel_map_waveex); i++)
		if (mask & (1U << i)) map[j++] = channel_map_waveex[i];
}

unsigned int channel_map_to_waveex(int channels, const channel_position_t *map, int *permutation)
{
	if (channels < 1 || channels > N_ELEMENTS(channel_map_waveex)) return 0;
	CHANNEL_MAP(default_map);
	if (!map) {
		channel_map_init_waveex(channels, 0, default_map);
		map = default_map;
	}
	if (!channel_map_valid(map)) return 0;
	int indexes[N_ELEMENTS(channel_map_waveex)];
	for (unsigned i = 0; i < N_ELEMENTS(indexes); i++) indexes[i] = -1;
	unsigned mask = 0;
	for (int channel = 0; channel < channels; channel++) {
		channel_position_t position = map[channel];
		if (channels == 1 && position == CHANNEL_POSITION_MONO) position = CHANNEL_POSITION_FRONT_CENTER;
		unsigned i;
		for (i = 0; i < N_ELEMENTS(channel_map_waveex); i++) if (channel_map_waveex[i] == position) break;
		if (i == N_ELEMENTS(channel_map_waveex) || indexes[i] >= 0) return 0;
		indexes[i] = channel;
		mask |= 1U << i;
	}
	if (permutation) {
		int channel = 0;
		for (unsigned i = 0; i < N_ELEMENTS(indexes); i++)
			if (indexes[i] >= 0) permutation[channel++] = indexes[i];
	}
	return mask;
}
