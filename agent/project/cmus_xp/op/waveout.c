/*
 * Copyright 2008-2013 Various Authors
 * Copyright 2007 dnk <dnk@bjum.net>
 *
 * Based on oss.c
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

#include "../op.h"
#include "../sf.h"
#include "../utils.h"
#include "../xmalloc.h"
#include "../debug.h"
#include "../windows/audio_devices.h"

#ifdef CMUS_WAVEOUT_TEST
#include "../windows/test_wave_api.h"
#else
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <mmsystem.h>
#include <mmreg.h>
#endif
#include <stdio.h>

static HWAVEOUT wave_out;
static sample_format_t waveout_sf;
static int buffer_size = 4096, buffer_count = 12;
static WAVEHDR *buffers;
static int buffer_idx, buffers_free, waveout_paused;
static ULONGLONG last_progress;
static char *device_selection;
static int channel_permutation[CHANNELS_MAX], reorder_channels;
#define FRAME_SIZE_ALIGN(x) (((x) / sf_get_frame_size(waveout_sf)) * sf_get_frame_size(waveout_sf))

static int waveout_result(const char *name, int rc)
{
    d_print("%s failed: WinMM error %d\n", name, rc);
    switch (rc) {
    case MMSYSERR_NODRIVER: case MMSYSERR_BADDEVICEID: case MMSYSERR_INVALHANDLE:
        errno = ENODEV; break;
    case MMSYSERR_ALLOCATED: case WAVERR_STILLPLAYING: errno = EBUSY; break;
    case MMSYSERR_NOMEM: errno = ENOMEM; break;
    default: errno = EIO; break;
    }
    return -OP_ERROR_ERRNO;
}
static int clean_buffers(void)
{
    for (int i = 0; i < buffer_count; i++) {
        WAVEHDR *hdr = &buffers[i];
        if (!(hdr->dwFlags & WHDR_DONE)) continue;
        int rc = waveOutUnprepareHeader(wave_out, hdr, sizeof(*hdr));
        if (rc != MMSYSERR_NOERROR) return waveout_result("waveOutUnprepareHeader", rc);
        hdr->dwFlags = 0; buffers_free++; last_progress = GetTickCount64();
    }
    return 0;
}
static int waveout_health(void)
{
    int rc = clean_buffers();
    if (rc < 0 || buffers_free == buffer_count || waveout_paused) return rc;
    MMTIME position = { .wType = TIME_BYTES };
    rc = waveOutGetPosition(wave_out, &position, sizeof(position));
    if (rc != MMSYSERR_NOERROR && rc != MMSYSERR_NOTSUPPORTED)
        return waveout_result("waveOutGetPosition", rc);
    ULONGLONG buffer_ms = (ULONGLONG)buffer_size * 1000 / sf_get_second_size(waveout_sf);
    ULONGLONG timeout = buffer_ms * 3 + 2000;
    if (timeout < 10000) timeout = 10000;
    if (GetTickCount64() - last_progress > timeout) { errno = ENODEV; return -OP_ERROR_ERRNO; }
    return 0;
}
static int waveout_open(sample_format_t sf, const channel_position_t *channel_map)
{
    if (wave_out) { errno = EBUSY; return -OP_ERROR_ERRNO; }
    unsigned bits = sf_get_bits(sf), channels = sf_get_channels(sf);
    if (!sf_get_rate(sf) || sf_get_bigendian(sf) || channels < 1 || channels > 18 ||
            !((bits == 8 && !sf_get_signed(sf)) ||
              ((bits == 16 || bits == 24 || bits == 32) && sf_get_signed(sf))))
        return -OP_ERROR_SAMPLE_FORMAT;
    unsigned mask = channel_map_to_waveex(channels, channel_map, channel_permutation);
    if (!mask) return -OP_ERROR_SAMPLE_FORMAT;
    reorder_channels = 0;
    for (unsigned i = 0; i < channels; i++) if (channel_permutation[i] != (int)i) reorder_channels = 1;
    unsigned device;
    if (win_waveout_device_id(device_selection, &device)) return -OP_ERROR_ERRNO;
    WAVEFORMATEXTENSIBLE format = { .Format = {
        .wFormatTag = WAVE_FORMAT_PCM, .nChannels = sf_get_channels(sf),
        .nSamplesPerSec = sf_get_rate(sf), .wBitsPerSample = sf_get_bits(sf),
        .nAvgBytesPerSec = sf_get_second_size(sf), .nBlockAlign = sf_get_frame_size(sf)
    }};
    if (bits > 16 || channels > 2 || (channels == 1 ? mask != 4 : mask != 3)) {
        format.Format.wFormatTag = WAVE_FORMAT_EXTENSIBLE;
        format.Format.cbSize = sizeof(format) - sizeof(format.Format);
        format.Samples.wValidBitsPerSample = sf_get_bits(sf);
        format.dwChannelMask = mask;
        format.SubFormat = (GUID){ 1, 0, 0x10, {0x80,0,0,0xaa,0,0x38,0x9b,0x71} };
    }
    HWAVEOUT opened = NULL;
    int rc = waveOutOpen(&opened, device, &format.Format, 0, 0, CALLBACK_NULL);
    if (rc == WAVERR_BADFORMAT) return -OP_ERROR_SAMPLE_FORMAT;
    if (rc != MMSYSERR_NOERROR) return waveout_result("waveOutOpen", rc);
    wave_out = opened;
    d_print("opened output device %u: %u Hz, %u bits, %u channels, speaker mask 0x%x\n",
        device, sf_get_rate(sf), bits, channels, mask);
    for (int i = 0; i < buffer_count; i++) buffers[i].dwFlags = 0;
    buffer_idx = 0; buffers_free = buffer_count; waveout_sf = sf;
    waveout_paused = 0; last_progress = GetTickCount64();
    return 0;
}
static int waveout_drop(void)
{
    if (!wave_out) return 0;
    int rc = waveOutReset(wave_out);
    if (rc != MMSYSERR_NOERROR) return waveout_result("waveOutReset", rc);
    rc = clean_buffers(); buffer_idx = 0; last_progress = GetTickCount64();
    return rc;
}
static int waveout_close(void)
{
    if (!wave_out) return 0;
    int drain = 1;
    /* Natural EOF/format changes drain; explicit stop/seek call drop first. */
    if (waveout_paused && buffers_free < buffer_count) {
        int rc = waveOutRestart(wave_out);
        if (rc == MMSYSERR_NOERROR) waveout_paused = 0;
        else { waveout_drop(); drain = 0; }
    }
    while (drain && buffers_free < buffer_count) {
        if (waveout_health() < 0) { waveout_drop(); break; }
        if (buffers_free < buffer_count) Sleep(5);
    }
    int rc = waveOutClose(wave_out);
    if (rc != MMSYSERR_NOERROR && rc != MMSYSERR_NODRIVER && rc != MMSYSERR_INVALHANDLE)
        return waveout_result("waveOutClose", rc);
    wave_out = NULL; waveout_paused = 0;
    for (int i = 0; i < buffer_count; i++) buffers[i].dwFlags = 0;
    buffers_free = buffer_count;
    /* A vanished/invalid handle is already closed. Reported write/health
     * errors remain visible, but the lifecycle must permit a fresh device. */
    if (rc != MMSYSERR_NOERROR) waveout_result("waveOutClose", rc);
    return 0;
}
static int waveout_init(void)
{
    buffers = xnew0(WAVEHDR, buffer_count);
    for (int i = 0; i < buffer_count; i++) buffers[i].lpData = xmalloc(buffer_size);
    return 0;
}
static int waveout_exit(void)
{
    if (wave_out) {
        waveout_drop();
        if (waveout_close() < 0 && wave_out) return -OP_ERROR_ERRNO;
    }
    for (int i = 0; i < buffer_count; i++) free(buffers[i].lpData);
    free(buffers); buffers = NULL;
    free(device_selection); device_selection = NULL;
    return 0;
}
static int waveout_write(const char *buffer, int count)
{
    int written = 0;
    count = FRAME_SIZE_ALIGN(count);
    int status = waveout_health();
    if (status < 0) return status;
    while (count > 0) {
        WAVEHDR *hdr = &buffers[buffer_idx];
        if (hdr->dwFlags != 0) break;
        int len = FRAME_SIZE_ALIGN(min_i(count, buffer_size));
        hdr->dwBufferLength = len;
        if (!reorder_channels) memcpy(hdr->lpData, buffer + written, len);
        else {
            unsigned channels = sf_get_channels(waveout_sf), bytes = sf_get_sample_size(waveout_sf);
            unsigned frame_size = sf_get_frame_size(waveout_sf);
            for (unsigned offset = 0; offset < (unsigned)len; offset += frame_size)
                for (unsigned channel = 0; channel < channels; channel++)
                    memcpy(hdr->lpData + offset + channel * bytes,
                        buffer + written + offset + channel_permutation[channel] * bytes, bytes);
        }
        int rc = waveOutPrepareHeader(wave_out, hdr, sizeof(*hdr));
        if (rc != MMSYSERR_NOERROR) return written ? written : waveout_result("waveOutPrepareHeader", rc);
        rc = waveOutWrite(wave_out, hdr, sizeof(*hdr));
        if (rc != MMSYSERR_NOERROR) {
            waveOutUnprepareHeader(wave_out, hdr, sizeof(*hdr)); hdr->dwFlags = 0;
            return written ? written : waveout_result("waveOutWrite", rc);
        }
        if (buffers_free == buffer_count) last_progress = GetTickCount64();
        written += len; count -= len; buffer_idx = (buffer_idx + 1) % buffer_count; buffers_free--;
    }
    return written;
}
static int waveout_pause(void)
{
    int rc = waveOutPause(wave_out);
    if (rc != MMSYSERR_NOERROR) return waveout_result("waveOutPause", rc);
    waveout_paused = 1; return 0;
}
static int waveout_unpause(void)
{
    int rc = waveOutRestart(wave_out);
    if (rc != MMSYSERR_NOERROR) return waveout_result("waveOutRestart", rc);
    waveout_paused = 0; last_progress = GetTickCount64(); return 0;
}
static int waveout_buffer_space(void)
{
    int rc = waveout_health();
    return rc < 0 ? rc : buffers_free * FRAME_SIZE_ALIGN(buffer_size);
}

static int waveout_set_buffer_common(long int val, int *dst)
{
	int reinit = 0;
    if (wave_out) { errno = EBUSY; return -OP_ERROR_ERRNO; }

	if (buffers) {
		char *selected = device_selection;
		device_selection = NULL;
		waveout_exit();
		device_selection = selected;
		reinit = 1;
	}
	*dst = val;

	if (reinit) {
		waveout_init();
	}

	return 0;
}

static int waveout_set_buffer_size(const char *val)
{
	long int ival;
	if (str_to_int(val, &ival) || ival < 4096 || ival > 65536) {
		errno = EINVAL;
		return -OP_ERROR_ERRNO;
	}
	return waveout_set_buffer_common(ival, &buffer_size);
}

static int waveout_set_buffer_count(const char *val)
{
	long int ival;
	if (str_to_int(val, &ival) || ival < 2 || ival > 64) {
		errno = EINVAL;
		return -OP_ERROR_ERRNO;
	}
	return waveout_set_buffer_common(ival, &buffer_count);
}

static int waveout_get_buffer_size(char **val)
{
	*val = xnew(char, 22);
	snprintf(*val, 22, "%d", buffer_size);
	return 0;
}

static int waveout_get_buffer_count(char **val)
{
	*val = xnew(char, 22);
	snprintf(*val, 22, "%d", buffer_count);
	return 0;
}

static int waveout_set_device(const char *value)
{
    if (wave_out) { errno = EBUSY; return -OP_ERROR_ERRNO; }
    if (strlen(value) >= 4096) { errno = EINVAL; return -OP_ERROR_ERRNO; }
    char *copy = xstrdup(*value ? value : "default");
    free(device_selection); device_selection = copy;
    return 0;
}
static int waveout_get_device(char **value)
{
    *value = xstrdup(device_selection ? device_selection : "default");
    return 0;
}

const struct output_plugin_ops op_pcm_ops = {
	.init = waveout_init,
	.exit = waveout_exit,
	.open = waveout_open,
	.close = waveout_close,
	.drop = waveout_drop,
	.write = waveout_write,
	.pause = waveout_pause,
	.unpause = waveout_unpause,
	.buffer_space = waveout_buffer_space,
};

const struct output_plugin_opt op_pcm_options[] = {
	OPT(waveout, buffer_size),
	OPT(waveout, buffer_count),
	OPT(waveout, device),
	{ NULL },
};

const int op_priority = 0;
const unsigned op_abi_version = OP_ABI_VERSION;
