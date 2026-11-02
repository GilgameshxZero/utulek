/*
 * Copyright 2008-2013 Various Authors
 * Copyright 2007 Kevin Ko <kevin.s.ko@gmail.com>
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
#include "../xmalloc.h"
#include "../debug.h"
#include "../utils.h"
#include "../comment.h"
#include <pthread.h>
#ifdef HAVE_CONFIG
#include "config/ffmpeg.h"
#endif

#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavformat/avio.h>
#include <libswresample/swresample.h>
#include <libavutil/opt.h>
#include <libavutil/channel_layout.h>
#include <libavutil/downmix_info.h>
#ifndef AVUTIL_MATHEMATICS_H
#include <libavutil/mathematics.h>
#endif

struct ffmpeg_private {
	AVCodecContext *codec_ctx;
	AVFormatContext *format_ctx;
	AVCodec const *codec;
	SwrContext *swr;
	int stream_index;

	AVPacket *pkt;
	AVFrame *frame;
	double seek_ts;
	int64_t skip_samples;

	/* A buffer to hold swr_convert()-ed samples */
	AVFrame *swr_frame;
	int swr_frame_samples_cap;
	int swr_frame_start;
	int direct_pcm;
	int decoder_eof;
	sample_format_t output_sf;
	enum AVSampleFormat output_sample_fmt;
	int downmix_checked;

	/* Bitrate estimation */
	unsigned long curr_size;
	unsigned long curr_duration;
};

static const char *ffmpeg_errmsg(int err)
{
	static _Thread_local char errstr[AV_ERROR_MAX_STRING_SIZE];
	av_strerror(err, errstr, AV_ERROR_MAX_STRING_SIZE);
	return errstr;
}

static void ffmpeg_init_once(void)
{
	av_log_set_level(AV_LOG_QUIET);

#if LIBAVFORMAT_VERSION_INT < AV_VERSION_INT(58, 9, 100)
	/* We could register decoders explicitly to save memory, but we have to
	 * be careful about compatibility. */
	av_register_all();
#endif
}

static void ffmpeg_init(void)
{
	static pthread_once_t once = PTHREAD_ONCE_INIT;
	pthread_once(&once, ffmpeg_init_once);
}

static int ffmpeg_open_input(struct input_plugin_data *ip_data,
		struct ffmpeg_private *priv)
{
	AVFormatContext *ic = NULL;
	AVCodecContext *cc = NULL;
	AVCodecParameters *cp = NULL;
	AVCodec const *codec = NULL;
	int stream_index = -1;

	int err;
	int res = avformat_open_input(&ic, ip_data->filename, NULL, NULL);
	if (res < 0) {
		err = -IP_ERROR_FILE_FORMAT;
		goto err;
	}

	res = avformat_find_stream_info(ic, NULL);
	if (res < 0) {
		d_print("unable to find stream info\n");
		err = -IP_ERROR_FILE_FORMAT;
		goto err;
	}

	/* Prefer the default, decodable audio track, including in video containers. */
	stream_index = av_find_best_stream(ic, AVMEDIA_TYPE_AUDIO, -1, -1, &codec, 0);
	if (stream_index < 0) {
		d_print("could not find audio stream\n");
		err = -IP_ERROR_UNSUPPORTED_FILE_TYPE;
		goto err_silent;
	}
	cp = ic->streams[stream_index]->codecpar;
	/* Let demuxers skip unused video/subtitle/alternate audio packets. */
	for (unsigned i = 0; i < ic->nb_streams; i++)
		ic->streams[i]->discard = i == (unsigned)stream_index ? AVDISCARD_DEFAULT : AVDISCARD_ALL;
	cc = avcodec_alloc_context3(codec);
    if (!cc) { err = -IP_ERROR_ERRNO; errno = ENOMEM; goto err_silent; }
    res = avcodec_parameters_to_context(cc, cp);
    if (res < 0) { err = -IP_ERROR_FILE_FORMAT; goto err; }

	res = avcodec_open2(cc, codec, NULL);
	if (res < 0) {
		d_print("could not open codec (id: %d, name: %s)\n",
				cc->codec_id, avcodec_get_name(cc->codec_id));
		err = -IP_ERROR_UNSUPPORTED_FILE_TYPE;
		goto err;
	}

	priv->format_ctx = ic;
	priv->codec_ctx = cc;
	priv->codec = codec;
	priv->stream_index = stream_index;
	return 0;
err:
	d_print("%s\n", ffmpeg_errmsg(res));
err_silent:
	avcodec_free_context(&cc);
	avformat_close_input(&ic);
	return err;
}

static void ffmpeg_set_sf_and_swr_opts(SwrContext *swr, AVCodecContext *cc,
		sample_format_t *sf_out, enum AVSampleFormat *out_sample_fmt)
{
	int out_sample_rate = min_u(cc->sample_rate, 384000);
	sample_format_t sf = sf_rate(out_sample_rate) | sf_host_endian();
	av_opt_set_int(swr, "in_sample_rate", cc->sample_rate, 0);
	av_opt_set_int(swr, "out_sample_rate", out_sample_rate, 0);

	switch (cc->sample_fmt) {
		case AV_SAMPLE_FMT_DBL: case AV_SAMPLE_FMT_DBLP:
		case AV_SAMPLE_FMT_S64: case AV_SAMPLE_FMT_S64P:
		case AV_SAMPLE_FMT_FLT: case AV_SAMPLE_FMT_FLTP:
		case AV_SAMPLE_FMT_S32: case AV_SAMPLE_FMT_S32P:
			sf |= sf_bits(32) | sf_signed(1);
			*out_sample_fmt = AV_SAMPLE_FMT_S32;
			break;
		default:
			sf |= sf_bits(16) | sf_signed(1);
			*out_sample_fmt = AV_SAMPLE_FMT_S16;
	}
	av_opt_set_sample_fmt(swr, "in_sample_fmt", cc->sample_fmt, 0);
	av_opt_set_sample_fmt(swr, "out_sample_fmt", *out_sample_fmt, 0);

#if LIBAVCODEC_VERSION_INT >= AV_VERSION_INT(59, 24, 100)
	    int out_channels = cc->ch_layout.nb_channels;
#ifdef CMUS_FFMPEG_STEREO
    /* Output policy, independent of OS: mono expansion and surround downmix. */
    out_channels = 2;
#endif
    sf |= sf_channels(out_channels);

	if (cc->ch_layout.order == AV_CHANNEL_ORDER_UNSPEC)
		av_channel_layout_default(&cc->ch_layout, cc->ch_layout.nb_channels);
	av_opt_set_chlayout(swr, "in_chlayout", &cc->ch_layout, 0);
	    AVChannelLayout output_layout;
    memset(&output_layout, 0, sizeof(output_layout));
#ifdef CMUS_FFMPEG_STEREO
    av_channel_layout_default(&output_layout, out_channels);
#else
    av_channel_layout_copy(&output_layout, &cc->ch_layout);
#endif
    av_opt_set_chlayout(swr, "out_chlayout", &output_layout, 0);
    av_channel_layout_uninit(&output_layout);
#else
	int out_channels = cc->channels;
#ifdef CMUS_FFMPEG_STEREO
	out_channels = 2;
#endif
	sf |= sf_channels(out_channels);

	av_opt_set_int(swr, "in_channel_layout",
			av_get_default_channel_layout(cc->channels), 0);
	av_opt_set_int(swr, "out_channel_layout",
			av_get_default_channel_layout(out_channels), 0);
#endif

	*sf_out = sf;
}

static int ffmpeg_init_swr_frame(struct ffmpeg_private *priv,
		sample_format_t sf, enum AVSampleFormat out_sample_fmt)
{
	AVFrame *frame = av_frame_alloc();
	if (!frame) {
		errno = ENOMEM;
		return -IP_ERROR_ERRNO;
	}

#if LIBAVCODEC_VERSION_INT >= AV_VERSION_INT(59, 24, 100)
	av_channel_layout_default(&frame->ch_layout, sf_get_channels(sf));
#else
	frame->channel_layout = av_get_default_channel_layout(sf_get_channels(sf));
#endif

	frame->sample_rate = sf_get_rate(sf);
	frame->format = out_sample_fmt;
	priv->output_sf = sf;
	priv->output_sample_fmt = out_sample_fmt;

	/* Allocate PCM lazily, using swr_get_out_samples() for each frame. */
	priv->swr_frame = frame;
	return 0;
}

static void ffmpeg_free(struct ffmpeg_private *priv)
{
	avcodec_free_context(&priv->codec_ctx);
	avformat_close_input(&priv->format_ctx);

	swr_free(&priv->swr);

	av_frame_free(&priv->frame);
	av_packet_free(&priv->pkt);
	av_frame_free(&priv->swr_frame);
}

static int ffmpeg_open(struct input_plugin_data *ip_data)
{
	struct ffmpeg_private priv;
	enum AVSampleFormat out_sample_fmt;
	memset(&priv, 0, sizeof(struct ffmpeg_private));

	ffmpeg_init();

	int err = ffmpeg_open_input(ip_data, &priv);
	if (err < 0)
		return err;

    priv.pkt = av_packet_alloc();
	priv.frame = av_frame_alloc();
	priv.seek_ts = -1;

    priv.swr = swr_alloc();
    if (!priv.pkt || !priv.frame || !priv.swr) {
        ffmpeg_free(&priv); errno = ENOMEM;
        return -IP_ERROR_ERRNO;
    }
	ffmpeg_set_sf_and_swr_opts(priv.swr, priv.codec_ctx,
			&ip_data->sf, &out_sample_fmt);
    /* Packed PCM with identical format/layout/rate needs no conversion/copy. */
#if LIBAVCODEC_VERSION_INT >= AV_VERSION_INT(59, 24, 100)
    AVChannelLayout output_layout;
    memset(&output_layout, 0, sizeof(output_layout));
#ifdef CMUS_FFMPEG_STEREO
    av_channel_layout_default(&output_layout, sf_get_channels(ip_data->sf));
#else
    av_channel_layout_copy(&output_layout, &priv.codec_ctx->ch_layout);
#endif
    priv.direct_pcm = av_channel_layout_compare(&output_layout, &priv.codec_ctx->ch_layout) == 0;
    av_channel_layout_uninit(&output_layout);
#else
    priv.direct_pcm = priv.codec_ctx->channel_layout ==
        av_get_default_channel_layout(sf_get_channels(ip_data->sf));
#endif
    priv.direct_pcm = priv.direct_pcm && priv.codec_ctx->sample_fmt == out_sample_fmt &&
        priv.codec_ctx->sample_rate == sf_get_rate(ip_data->sf);
    int swr_error = swr_init(priv.swr);
    if (swr_error < 0) {
        ffmpeg_free(&priv);
        return -IP_ERROR_SAMPLE_FORMAT;
    }

	err = ffmpeg_init_swr_frame(&priv, ip_data->sf, out_sample_fmt);
	if (err < 0) {
		ffmpeg_free(&priv);
		return err;
	}

#if LIBAVCODEC_VERSION_INT >= AV_VERSION_INT(59, 24, 100)
	channel_map_init_waveex(sf_get_channels(ip_data->sf),
            sf_get_channels(ip_data->sf) == 2 ? 3 : priv.codec_ctx->ch_layout.u.mask, ip_data->channel_map);
#else
	channel_map_init_waveex(priv.codec_ctx->channels,
			priv.codec_ctx->channel_layout, ip_data->channel_map);
#endif

	ip_data->private = xnew(struct ffmpeg_private, 1);
	memcpy(ip_data->private, &priv, sizeof(struct ffmpeg_private));
	return 0;
}

static int ffmpeg_close(struct input_plugin_data *ip_data)
{
	ffmpeg_free(ip_data->private);
	free(ip_data->private);
	ip_data->private = NULL;
	return 0;
}

static int64_t ffmpeg_calc_skip_samples(struct ffmpeg_private *priv)
{
	int64_t ts;
	if (priv->frame->pts != AV_NOPTS_VALUE) {
		ts = priv->frame->pts;
	} else if (priv->frame->pkt_dts != AV_NOPTS_VALUE) {
		ts = priv->frame->pkt_dts;
	} else {
		d_print("AVFrame.pts and AVFrame.pkt_dts are unset\n");
		return -1;
	}

	AVStream *s = priv->format_ctx->streams[priv->stream_index];
	int64_t start = s->start_time == AV_NOPTS_VALUE ? 0 : s->start_time;
	double frame_ts = ((double)ts - (double)start) * av_q2d(s->time_base);

	d_print("seek_ts: %.6fs, frame_ts: %.6fs\n", priv->seek_ts, frame_ts);

	if (frame_ts >= priv->seek_ts)
		return 0;
	double skip = (priv->seek_ts - frame_ts) * priv->frame->sample_rate;
	if (!isfinite(skip) || skip >= (double)INT64_MAX)
		return -1;
	return skip;
}

static void ffmpeg_skip_frame_part(struct ffmpeg_private *priv)
{
	if (priv->skip_samples >= priv->frame->nb_samples) {
		d_print("skipping frame: %d samples\n",
				priv->frame->nb_samples);
		priv->skip_samples -= priv->frame->nb_samples;
		priv->frame->nb_samples = 0;
		return;
	}

	int bps = av_get_bytes_per_sample(priv->frame->format);
#if LIBAVCODEC_VERSION_INT >= AV_VERSION_INT(59, 24, 100)
	int channels = priv->codec_ctx->ch_layout.nb_channels;
#else
	int channels = priv->codec_ctx->channels;
#endif

	priv->frame->nb_samples -= priv->skip_samples;

	/* Just modify frame's data pointer because it's throw-away */
	if (av_sample_fmt_is_planar(priv->frame->format)) {
		for (int i = 0; i < channels; i++)
			priv->frame->extended_data[i] += priv->skip_samples * bps;
	} else {
		priv->frame->extended_data[0] += priv->skip_samples * channels * bps;
	}
	d_print("skipping %lld samples\n", (long long)priv->skip_samples);
	priv->skip_samples = 0;
}

/*
 * return:
 *   <0 - error
 *    0 - eof
 *   >0 - ok
 */
static int ffmpeg_get_frame(struct ffmpeg_private *priv)
{
	int res;
	int corrupt_packets = 0;
retry:
	av_frame_unref(priv->frame);
	res = avcodec_receive_frame(priv->codec_ctx, priv->frame);
	if (res == AVERROR(EAGAIN)) {
		av_packet_unref(priv->pkt);
		res = av_read_frame(priv->format_ctx, priv->pkt);
		if (res == AVERROR_EOF) {
			/* Decoders may retain the final audio frame until drained. */
			res = avcodec_send_packet(priv->codec_ctx, NULL);
			if (res == 0 || res == AVERROR(EAGAIN))
				goto retry;
			goto err;
		}
		if (res < 0)
			goto err;

		if (priv->pkt->stream_index != priv->stream_index)
			goto retry;

		priv->curr_size += priv->pkt->size;
		priv->curr_duration += priv->pkt->duration;

		res = avcodec_send_packet(priv->codec_ctx, priv->pkt);
		if (res == 0 || res == AVERROR(EAGAIN))
			goto retry;
	}
	if (res < 0)
		goto err;
	priv->swr_frame_start = 0;

	if (priv->seek_ts >= 0) {
		priv->skip_samples = ffmpeg_calc_skip_samples(priv);
		if (priv->skip_samples >= 0)
			priv->seek_ts = -1;
	}

	if (priv->skip_samples > 0) {
		ffmpeg_skip_frame_part(priv);
		if (priv->frame->nb_samples == 0)
			goto retry;
	}
	return 1;
err:
	if (res == AVERROR_EOF)
		return 0;
	/* Seeking can begin inside an audio frame (notably MPEG-PS). Skip
	 * recoverable damaged packets, with a bound for malformed inputs. */
	if (res == AVERROR_INVALIDDATA && ++corrupt_packets <= 64)
		goto retry;
	d_print("%s\n", ffmpeg_errmsg(res));
	return -IP_ERROR_INTERNAL;
}

static int ffmpeg_convert_frame(struct ffmpeg_private *priv, int drain)
{
	if (!drain && !priv->downmix_checked) {
		/* Respect the mastering metadata for surround soundtracks. */
		AVFrameSideData *sd = av_frame_get_side_data(priv->frame, AV_FRAME_DATA_DOWNMIX_INFO);
		if (sd && sf_get_channels(priv->output_sf) == 2) {
			const AVDownmixInfo *di = (const AVDownmixInfo *)sd->data;
			int matrix = AV_MATRIX_ENCODING_NONE;
			double center = di->center_mix_level, surround = di->surround_mix_level;
			if (di->preferred_downmix_type == AV_DOWNMIX_TYPE_LTRT ||
					di->preferred_downmix_type == AV_DOWNMIX_TYPE_DPLII) {
				matrix = di->preferred_downmix_type == AV_DOWNMIX_TYPE_LTRT ?
					AV_MATRIX_ENCODING_DOLBY : AV_MATRIX_ENCODING_DPLII;
				center = di->center_mix_level_ltrt;
				surround = di->surround_mix_level_ltrt;
			}
			swr_close(priv->swr);
			av_opt_set_double(priv->swr, "clev", center, 0);
			av_opt_set_double(priv->swr, "slev", surround, 0);
			av_opt_set_double(priv->swr, "lfe_mix_level", di->lfe_mix_level, 0);
			av_opt_set_int(priv->swr, "matrix_encoding", matrix, 0);
			if (swr_init(priv->swr) < 0)
				return -IP_ERROR_SAMPLE_FORMAT;
		}
		priv->downmix_checked = 1;
	}
	int samples = swr_get_out_samples(priv->swr, drain ? 0 : priv->frame->nb_samples);
	if (samples < 0)
		return -IP_ERROR_INTERNAL;
	if (samples > priv->swr_frame_samples_cap) {
		av_frame_unref(priv->swr_frame);
#if LIBAVCODEC_VERSION_INT >= AV_VERSION_INT(59, 24, 100)
#ifdef CMUS_FFMPEG_STEREO
		av_channel_layout_default(&priv->swr_frame->ch_layout,
			sf_get_channels(priv->output_sf));
#else
		av_channel_layout_copy(&priv->swr_frame->ch_layout, &priv->codec_ctx->ch_layout);
#endif
#else
		priv->swr_frame->channel_layout = av_get_default_channel_layout(sf_get_channels(priv->output_sf));
#endif
		priv->swr_frame->format = priv->output_sample_fmt;
		priv->swr_frame->sample_rate = sf_get_rate(priv->output_sf);
		priv->swr_frame->nb_samples = samples;
		if (av_frame_get_buffer(priv->swr_frame, 0) < 0)
			return -IP_ERROR_INTERNAL;
		priv->swr_frame_samples_cap = samples;
	}
	int res = swr_convert(priv->swr,
			priv->swr_frame->extended_data,
			priv->swr_frame_samples_cap,
			drain ? NULL : (const uint8_t **)priv->frame->extended_data,
			drain ? 0 : priv->frame->nb_samples);
	if (res >= 0) {
		priv->swr_frame->nb_samples = res;
		priv->swr_frame_start = 0;
		return res;
	}
	d_print("%s\n", ffmpeg_errmsg(res));
	return -IP_ERROR_INTERNAL;
}

static int ffmpeg_read(struct input_plugin_data *ip_data, char *buffer, int count)
{
	struct ffmpeg_private *priv = ip_data->private;
	int written = 0;
	int res;

	count /= sf_get_frame_size(ip_data->sf);

	while (count) {
		if (priv->swr_frame->nb_samples == 0) {
			res = priv->decoder_eof ? 0 : ffmpeg_get_frame(priv);
			if (res < 0)
				return res;
			priv->decoder_eof = res == 0;
			if (priv->direct_pcm) {
				if (priv->decoder_eof)
					break;
				priv->swr_frame->nb_samples = priv->frame->nb_samples;
			} else {
				res = ffmpeg_convert_frame(priv, priv->decoder_eof);
				if (res < 0)
					return res;
				if (!res) {
					if (priv->decoder_eof)
						break;
					continue;
				}
			}
		}

		int copy_frames = min_i(count, priv->swr_frame->nb_samples);
		int copy_bytes = copy_frames * sf_get_frame_size(ip_data->sf);
		const uint8_t *dst = (priv->direct_pcm ? priv->frame->extended_data[0] :
			priv->swr_frame->extended_data[0]) + priv->swr_frame_start;
		memcpy(buffer + written, dst, copy_bytes);

		priv->swr_frame->nb_samples -= copy_frames;
		priv->swr_frame_start += copy_bytes;
		count -= copy_frames;
		written += copy_bytes;
	}
	return written;
}

static int ffmpeg_seek(struct input_plugin_data *ip_data, double offset)
{
	if (!isfinite(offset) || offset < 0)
		return -IP_ERROR_FUNCTION_NOT_SUPPORTED;
	if (offset == 0) {
		/* Seeking to timestamp zero can skip negative-timestamp priming packets
		 * (AAC/Opus), and some codecs retain state after flush. Reopen only at
		 * the beginning to restore exactly the initial demux/decoder state. */
		struct input_plugin_data reopened = *ip_data;
		reopened.private = NULL;
		int res = ffmpeg_open(&reopened);
		if (res < 0)
			return res;
		ffmpeg_close(ip_data);
		*ip_data = reopened;
		return 0;
	}
	struct ffmpeg_private *priv = ip_data->private;
	AVStream *st = priv->format_ctx->streams[priv->stream_index];

	double timestamp = offset / av_q2d(st->time_base);
	if (!isfinite(timestamp) || timestamp >= (double)INT64_MAX ||
			(st->start_time > 0 && timestamp >= (double)(INT64_MAX - st->start_time)))
		return -IP_ERROR_FUNCTION_NOT_SUPPORTED;
	int64_t ts = timestamp;
	if (offset > 0 && st->start_time != AV_NOPTS_VALUE)
		ts += st->start_time;

	int ret = avformat_seek_file(priv->format_ctx,
			priv->stream_index, INT64_MIN, ts, ts, 0);
	/* Some local demuxers implement the older timestamp-seek interface. */
	if (ret < 0)
		ret = av_seek_frame(priv->format_ctx, priv->stream_index, ts, AVSEEK_FLAG_BACKWARD);
	if (ret < 0)
		return -IP_ERROR_FUNCTION_NOT_SUPPORTED;
	priv->seek_ts = offset > 0 ? offset : -1;
	priv->skip_samples = 0;
	priv->curr_size = priv->curr_duration = 0;

	priv->swr_frame->nb_samples = 0;
	priv->swr_frame_start = 0;
	priv->decoder_eof = 0;
	priv->downmix_checked = 0;
	av_packet_unref(priv->pkt);
	av_frame_unref(priv->frame);
	avcodec_flush_buffers(priv->codec_ctx);
	/* Reinitialize: a zero-output conversion does not clear delayed samples. */
	swr_close(priv->swr);
	if (swr_init(priv->swr) < 0)
		return -IP_ERROR_INTERNAL;
	return 0;
}

static void ffmpeg_read_metadata(struct growing_keyvals *c, AVDictionary *metadata)
{
	AVDictionaryEntry *tag = NULL;

	while ((tag = av_dict_get(metadata, "", tag, AV_DICT_IGNORE_SUFFIX))) {
		if (tag->value[0])
			comments_add_const(c, tag->key, tag->value);
	}
}

static int ffmpeg_read_comments(struct input_plugin_data *ip_data,
		struct keyval **comments)
{
	struct ffmpeg_private *priv = ip_data->private;
	AVFormatContext *ic = priv->format_ctx;

	GROWING_KEYVALS(c);

	ffmpeg_read_metadata(&c, ic->metadata);
	ffmpeg_read_metadata(&c, ic->streams[priv->stream_index]->metadata);

	keyvals_terminate(&c);
	*comments = c.keyvals;

	return 0;
}

static int ffmpeg_duration(struct input_plugin_data *ip_data)
{
	struct ffmpeg_private *priv = ip_data->private;
	AVStream *st = priv->format_ctx->streams[priv->stream_index];
	if (st->duration != AV_NOPTS_VALUE)
		return st->duration * av_q2d(st->time_base);
	if (priv->format_ctx->duration != AV_NOPTS_VALUE)
		return priv->format_ctx->duration / AV_TIME_BASE;
	return -IP_ERROR_FUNCTION_NOT_SUPPORTED;
}

static long ffmpeg_bitrate(struct input_plugin_data *ip_data)
{
	struct ffmpeg_private *priv = ip_data->private;
	long bitrate = priv->codec_ctx->bit_rate;
	/* A movie's aggregate bitrate mostly describes video, not this track. */
	if (!bitrate && priv->format_ctx->nb_streams == 1)
		bitrate = priv->format_ctx->bit_rate;
	return bitrate ? bitrate : -IP_ERROR_FUNCTION_NOT_SUPPORTED;
}

static long ffmpeg_current_bitrate(struct input_plugin_data *ip_data)
{
	struct ffmpeg_private *priv = ip_data->private;
	AVStream *st = priv->format_ctx->streams[priv->stream_index];
	long bitrate = -1;
	/* ape codec returns silly numbers */
	if (priv->codec->id == AV_CODEC_ID_APE)
		return -1;
	if (priv->curr_duration > 0) {
		double seconds = priv->curr_duration * av_q2d(st->time_base);
		bitrate = (8 * priv->curr_size) / seconds;
		priv->curr_size = 0;
		priv->curr_duration = 0;
	}
	return bitrate;
}

static char *ffmpeg_codec(struct input_plugin_data *ip_data)
{
	struct ffmpeg_private *priv = ip_data->private;
	return xstrdup(priv->codec->name);
}

static char *ffmpeg_codec_profile(struct input_plugin_data *ip_data)
{
	struct ffmpeg_private *priv = ip_data->private;
	const char *profile;
	profile = av_get_profile_name(priv->codec, priv->codec_ctx->profile);
	return profile ? xstrdup(profile) : NULL;
}

const struct input_plugin_ops ip_ops = {
	.open = ffmpeg_open,
	.close = ffmpeg_close,
	.read = ffmpeg_read,
	.seek = ffmpeg_seek,
	.read_comments = ffmpeg_read_comments,
	.duration = ffmpeg_duration,
	.bitrate = ffmpeg_bitrate,
	.bitrate_current = ffmpeg_current_bitrate,
	.codec = ffmpeg_codec,
	.codec_profile = ffmpeg_codec_profile
};

const int ip_priority = 30;
const char *const ip_extensions[] = {
	/* Keep this list aligned with scripts/build_audio_deps.py. */
	"aa", "aac", "ac3", "eac3", "aif", "aifc", "aiff", "amr", "ape", "au", "avi",
	"dff", "dsf", "dts", "dtshd",
	"fla", "flac", "m4a", "m4b", "mka", "mkv", "mp+", "mp2", "mp3",
	"mp4", "mpc", "mpeg", "mpg", "mpp", "mts", "m2ts", "ogg", "oga", "oma",
	"opus", "rm", "ra", "sbc", "shn", "spx", "tak", "thd", "truehd", "mlp",
	"ts", "tta", "flv", "wav", "webm", "weba", "wma", "wv",
#ifdef USE_FALLBACK_IP
	"*",
#endif
	NULL
};
const char *const ip_mime_types[] = { NULL };
const struct input_plugin_opt ip_options[] = { { NULL } };
const unsigned ip_abi_version = IP_ABI_VERSION;
