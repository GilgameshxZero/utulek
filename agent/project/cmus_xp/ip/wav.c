/*
 * Copyright 2008-2013 Various Authors
 * Copyright 2004-2005 Timo Hirvonen
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

#include "../ip.h"
#include "../file.h"
#include "../xmalloc.h"
#include "../debug.h"
#include "../utils.h"
#include "../comment.h"

#include <stdio.h>
#include <string.h>
#include <strings.h>
#include <errno.h>
#include <math.h>

#include <sys/types.h>
#include <unistd.h>

#define WAVE_FORMAT_PCM        0x0001U
#define WAVE_FORMAT_EXTENSIBLE 0xfffeU

#define WAVE_WRONG_HEADER 1

struct wav_private {
	off_t pcm_start;
	unsigned int pcm_size;
	unsigned int pos;

	/* size of one second of data */
	unsigned int sec_size;

	unsigned int frame_size;
};

static int read_chunk_header(int fd, char *name, unsigned int *size)
{
	int rc;
	char buf[8];

	rc = read_all(fd, buf, 8);
	if (rc == -1)
		return -IP_ERROR_ERRNO;
	if (rc != 8)
		return -IP_ERROR_FILE_FORMAT;
	*size = read_le32(buf + 4);
	memmove(name, buf, 4);
	return 0;
}

static int read_named_chunk_header(int fd, const char *name, unsigned int *size)
{
	int rc;
	char buf[4];

	rc = read_chunk_header(fd, buf, size);
	if (rc)
		return rc;
	if (memcmp(buf, name, 4))
		return WAVE_WRONG_HEADER;
	return 0;
}

static int find_chunk(int fd, const char *name, unsigned int *size)
{
	int rc;

	do {
		rc = read_named_chunk_header(fd, name, size);
		if (rc != WAVE_WRONG_HEADER)
			return rc;
		d_print("seeking %u\n", *size);
		if (lseek(fd, (off_t)*size + (*size & 1), SEEK_CUR) == -1) {
			d_print("seek failed\n");
			return -IP_ERROR_ERRNO;
		}
	} while (1);
}

static int wav_open(struct input_plugin_data *ip_data)
{
	struct wav_private *priv;
	char buf[4];
	char *fmt;
	int rc;
	unsigned int riff_size, fmt_size;
	int save;

	d_print("file: %s\n", ip_data->filename);
	priv = xnew(struct wav_private, 1);
	ip_data->private = priv;
	rc = read_named_chunk_header(ip_data->fd, "RIFF", &riff_size);
	if (rc == WAVE_WRONG_HEADER)
		rc = -IP_ERROR_FILE_FORMAT;
	if (rc)
		goto error_exit;
	rc = read_all(ip_data->fd, buf, 4);
	if (rc == -1) {
		rc = -IP_ERROR_ERRNO;
		goto error_exit;
	}
	if (rc != 4 || memcmp(buf, "WAVE", 4) != 0) {
		rc = -IP_ERROR_FILE_FORMAT;
		goto error_exit;
	}

	rc = find_chunk(ip_data->fd, "fmt ", &fmt_size);
	if (rc)
		goto error_exit;
	if (fmt_size < 16 || fmt_size > 65536) {
		d_print("size of \"fmt \" chunk is invalid (%u)\n", fmt_size);
		rc = -IP_ERROR_FILE_FORMAT;
		goto error_exit;
	}
	fmt = xnew(char, fmt_size);
	rc = read_all(ip_data->fd, fmt, fmt_size);
	if (rc == -1) {
		save = errno;
		free(fmt);
		errno = save;
		rc = -IP_ERROR_ERRNO;
		goto error_exit;
	}
	if (rc != fmt_size) {
		save = errno;
		free(fmt);
		errno = save;
		rc = -IP_ERROR_FILE_FORMAT;
		goto error_exit;
	}
	{
		unsigned int format_tag, channels, rate, bits, channel_mask = 0;

		format_tag = read_le16(fmt + 0);
		channels = read_le16(fmt + 2);
		rate = read_le32(fmt + 4);
		/* 4 bytes, bytes per second */
		/* 2 bytes, bytes per sample */
		bits = read_le16(fmt + 14);
		if (format_tag == WAVE_FORMAT_EXTENSIBLE) {
			unsigned int ext_size, valid_bits;
			if (fmt_size < 40) {
				free(fmt);
				d_print("size of \"fmt \" chunk is invalid (%u)\n", fmt_size);
				rc = -IP_ERROR_FILE_FORMAT;
				goto error_exit;
			}
			ext_size = read_le16(fmt + 16);
			if (ext_size < 22 || ext_size > fmt_size - 18) {
				free(fmt);
				d_print("size of \"fmt \" chunk extension is invalid (%u)\n", ext_size);
				rc = -IP_ERROR_FILE_FORMAT;
				goto error_exit;
			}
			valid_bits = read_le16(fmt + 18);
			if (!valid_bits || valid_bits > bits) {
				free(fmt);
				d_print("padded samples are not supported (%u != %u)\n", bits, valid_bits);
				rc = -IP_ERROR_FILE_FORMAT;
				goto error_exit;
			}
			if (valid_bits != bits) {
				free(fmt);
				rc = -IP_ERROR_UNSUPPORTED_FILE_TYPE;
				goto error_exit;
			}
			static const unsigned char pcm_tail[] = {0, 0, 0, 0, 0x10, 0, 0x80, 0, 0, 0xaa, 0, 0x38, 0x9b, 0x71};
			if (memcmp(fmt + 26, pcm_tail, sizeof(pcm_tail))) {
				free(fmt); rc = -IP_ERROR_UNSUPPORTED_FILE_TYPE; goto error_exit;
			}
			channel_mask = read_le32(fmt + 20);
			format_tag = read_le16(fmt + 24);
			/* ignore rest of extension tag */
		}
		free(fmt);

		if (format_tag != WAVE_FORMAT_PCM) {
			d_print("unsupported format tag %u, should be 1\n", format_tag);
			rc = -IP_ERROR_UNSUPPORTED_FILE_TYPE;
			goto error_exit;
		}
		if (!rate || channels < 1 || channels > CHANNELS_MAX) {
			rc = -IP_ERROR_FILE_FORMAT;
			goto error_exit;
		}
		if (rate > 384000) {
			/* The FFmpeg fallback can resample rates outside native PCM. */
			rc = -IP_ERROR_UNSUPPORTED_FILE_TYPE;
			goto error_exit;
		}
		if (bits != 8 && bits != 16 && bits != 24 && bits != 32) {
			rc = -IP_ERROR_SAMPLE_FORMAT;
			goto error_exit;
		}
		ip_data->sf = sf_channels(channels) | sf_rate(rate) | sf_bits(bits) |
			sf_signed(bits > 8);
		channel_map_init_waveex(channels, channel_mask, ip_data->channel_map);
	}

	if ((fmt_size & 1) && lseek(ip_data->fd, 1, SEEK_CUR) == -1) { rc = -IP_ERROR_ERRNO; goto error_exit; }
	rc = find_chunk(ip_data->fd, "data", &priv->pcm_size);
	if (rc)
		goto error_exit;
	priv->pcm_start = lseek(ip_data->fd, 0, SEEK_CUR);
	if (priv->pcm_start == -1) {
		rc = -IP_ERROR_ERRNO;
		goto error_exit;
	}

	priv->sec_size = sf_get_second_size(ip_data->sf);
	priv->frame_size = sf_get_frame_size(ip_data->sf);
	priv->pos = 0;

	d_print("pcm start: %u\n", (unsigned int)priv->pcm_start);
	d_print("pcm size: %u\n", priv->pcm_size);
	d_print("\n");
	d_print("sr: %d, ch: %d, bits: %d, signed: %d\n", sf_get_rate(ip_data->sf),
			sf_get_channels(ip_data->sf), sf_get_bits(ip_data->sf),
			sf_get_signed(ip_data->sf));

	/* clamp pcm_size to full frames (file might be corrupt or truncated) */
	priv->pcm_size -= priv->pcm_size % sf_get_frame_size(ip_data->sf);
	return 0;
error_exit:
	save = errno;
	free(priv);
	errno = save;
	return rc;
}

static int wav_close(struct input_plugin_data *ip_data)
{
	struct wav_private *priv;

	priv = ip_data->private;
	free(priv);
	ip_data->private = NULL;
	return 0;
}

static int wav_read(struct input_plugin_data *ip_data, char *buffer, int _count)
{
	struct wav_private *priv = ip_data->private;
	unsigned int count = _count;
	int rc;

	if (priv->pos == priv->pcm_size) {
		/* eof */
		return 0;
	}
	if (count > priv->pcm_size - priv->pos)
		count = priv->pcm_size - priv->pos;
	count -= count % priv->frame_size;
	if (!count) { errno = EINVAL; return -IP_ERROR_ERRNO; }
	rc = read_all(ip_data->fd, buffer, count);
	if (rc == -1) {
		d_print("read error\n");
		return -IP_ERROR_ERRNO;
	}
	if (rc == 0) {
		d_print("eof\n");
		return 0;
	}
	/* Concurrent truncation may leave a final partial sample frame. */
	rc -= rc % priv->frame_size;
	priv->pos += rc;
	return rc;
}

static int wav_seek(struct input_plugin_data *ip_data, double _offset)
{
	struct wav_private *priv = ip_data->private;
	unsigned int offset;

	if (!isfinite(_offset) || _offset < 0)
		return -IP_ERROR_FUNCTION_NOT_SUPPORTED;
	/* Round sample frames, not bytes followed by downward alignment. */
	double frames = _offset * sf_get_rate(ip_data->sf);
	if (!isfinite(frames) || frames > UINT32_MAX / priv->frame_size)
		return -IP_ERROR_FUNCTION_NOT_SUPPORTED;
	if (frames >= priv->pcm_size / priv->frame_size)
		offset = priv->pcm_size;
	else
		offset = (uint64_t)(frames + .5) * priv->frame_size;
	if (lseek(ip_data->fd, priv->pcm_start + offset, SEEK_SET) == -1)
		return -1;
	priv->pos = offset;
	return 0;
}

static struct {
	const char *old;
	const char *new;
} key_map[] = {
	{ "IART", "artist" },
	{ "ICMT", "comment" },
	{ "ICOP", "copyright" },
	{ "ICRD", "date" },
	{ "IGNR", "genre" },
	{ "INAM", "title" },
	{ "IPRD", "album" },
	{ "IPRT", "tracknumber" },
	{ "ISFT", "software" },
	{ NULL, NULL }
};

static const char *lookup_key(const char *key)
{
	int i;
	for (i = 0; key_map[i].old; i++) {
		if (!strcasecmp(key, key_map[i].old))
			return key_map[i].new;
	}
	return NULL;
}

static void wav_info_comments(int fd, uint64_t position, uint64_t end,
        struct growing_keyvals *comments, size_t *allocated)
{
    while (position <= end && end - position >= 8) {
        char id[5] = {0};
        unsigned size;
        if (lseek(fd, position, SEEK_SET) == -1 || read_chunk_header(fd, id, &size)) break;
        uint64_t next = position + 8 + (uint64_t)size + (size & 1);
        if (next > end) break;
        const char *key = lookup_key(id);
        if (key && size <= 1024 * 1024 && *allocated + size <= 8 * 1024 * 1024) {
            char *value = xnew(char, (size_t)size + 1);
            if (read_all(fd, value, size) != size) { free(value); break; }
            value[size] = 0;
            *allocated += size;
            comments_add(comments, key, value);
        }
        position = next;
    }
}

static int wav_read_comments(struct input_plugin_data *ip_data,
        struct keyval **comments)
{
    GROWING_KEYVALS(c);
    struct wav_private *priv = ip_data->private;
    off_t original = lseek(ip_data->fd, 0, SEEK_CUR);
    if (original == -1) return -1;
    uint64_t position = 12, end = priv->pcm_start - 8;
    size_t allocated = 0;
    while (position <= end && end - position >= 8) {
        char id[5] = {0};
        unsigned size;
        if (lseek(ip_data->fd, position, SEEK_SET) == -1 || read_chunk_header(ip_data->fd, id, &size)) break;
        uint64_t next = position + 8 + (uint64_t)size + (size & 1);
        if (next > end) break;
        if (!memcmp(id, "LIST", 4) && size >= 4) {
            char type[4];
            if (read_all(ip_data->fd, type, sizeof(type)) != sizeof(type)) break;
            if (!memcmp(type, "INFO", 4))
                wav_info_comments(ip_data->fd, position + 12, position + 8 + size, &c, &allocated);
        }
        position = next;
    }
    /* Malformed optional tags do not prevent playing otherwise valid PCM. */
    keyvals_terminate(&c);
    if (lseek(ip_data->fd, original, SEEK_SET) == -1) { keyvals_free(c.keyvals); return -1; }
    *comments = c.keyvals;
    return 0;
}


static int wav_duration(struct input_plugin_data *ip_data)
{
	struct wav_private *priv;
	int duration;

	priv = ip_data->private;
	duration = priv->pcm_size / priv->sec_size;
	return duration;
}

static long wav_bitrate(struct input_plugin_data *ip_data)
{
	sample_format_t sf = ip_data->sf;
	return sf_get_bits(sf) * sf_get_rate(sf) * sf_get_channels(sf);
}

static char *wav_codec(struct input_plugin_data *ip_data)
{
	char buf[16];
	snprintf(buf, 16, "pcm_%c%u%s",
			sf_get_signed(ip_data->sf) ? 's' : 'u',
			sf_get_bits(ip_data->sf),
			sf_get_bigendian(ip_data->sf) ? "be" : "le");

	return xstrdup(buf);
}

static char *wav_codec_profile(struct input_plugin_data *ip_data)
{
	return NULL;
}

const struct input_plugin_ops ip_ops = {
	.open = wav_open,
	.close = wav_close,
	.read = wav_read,
	.seek = wav_seek,
	.read_comments = wav_read_comments,
	.duration = wav_duration,
	.bitrate = wav_bitrate,
	.bitrate_current = wav_bitrate,
	.codec = wav_codec,
	.codec_profile = wav_codec_profile
};

const int ip_priority = 50;
const char * const ip_extensions[] = { "wav", NULL };
const char * const ip_mime_types[] = { NULL };
const struct input_plugin_opt ip_options[] = { { NULL } };
const unsigned ip_abi_version = IP_ABI_VERSION;
