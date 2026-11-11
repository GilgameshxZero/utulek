/* Native Windows CD-DA input using the CD-ROM device IOCTLs.
 * GPL-2.0-or-later. */
#include "../ip.h"
#include "../file.h"
#include "../xmalloc.h"
#include "../comment.h"
#include "../debug.h"
#include "../options.h"
#include "../discid.h"
#include "../utils.h"

#include <errno.h>
#include <stdio.h>
#include <string.h>
#include <winioctl.h>
#include <ntddcdrm.h>
#include <wchar.h>

#define CDDA_FRAME 2352
#define CDDA_RATE 44100
#define CDDA_FRAMES_PER_SEC 75

struct cdda_private {
	HANDLE handle;
	char *device;
	char *disc_id;
	int track;
	DWORD first_sector;
	DWORD last_sector;
	DWORD current_sector;
	unsigned char frame[CDDA_FRAME];
	unsigned int used;
};

static int drive_path(const char *device, wchar_t *out, size_t size)
{
	wchar_t *wide;
	if (!device || !*device)
		return -1;
	if (device[0] == '\\' && device[1] == '\\')
		wide = win_wide(device);
	else if (device[0] && device[1] == ':')
	{
		char path[64];
		snprintf(path, sizeof(path), "\\\\.\\%c:", device[0]);
		wide = win_wide(path);
	} else
		wide = win_wide(device);
	if (!wide)
		return -1;
	if (wcslen(wide) + 1 > size) {
		free(wide);
		return -1;
	}
	wcscpy(out, wide);
	free(wide);
	return 0;
}

static int open_drive(const char *device, HANDLE *handle)
{
	wchar_t path[64];
	if (drive_path(device, path, sizeof(path) / sizeof(*path)))
		return -1;
	*handle = CreateFileW(path, GENERIC_READ,
			FILE_SHARE_READ | FILE_SHARE_WRITE, NULL, OPEN_EXISTING, 0, NULL);
	return *handle == INVALID_HANDLE_VALUE ? -1 : 0;
}

static int read_toc(HANDLE handle, CDROM_TOC *toc)
{
	DWORD used;
	if (!DeviceIoControl(handle, IOCTL_CDROM_READ_TOC, NULL, 0,
				toc, sizeof(*toc), &used, NULL))
		return -1;
	if (used < 4 || toc->LastTrack < toc->FirstTrack)
		return -1;
	return 0;
}

static DWORD track_address(const TRACK_DATA *track)
{
	return ((DWORD)track->Address[0] << 24) | ((DWORD)track->Address[1] << 16) |
		((DWORD)track->Address[2] << 8) | track->Address[3];
}

static int find_track(const CDROM_TOC *toc, int track, int *audio,
		DWORD *first, DWORD *last)
{
	DWORD leadout = 0;
	int found = 0;
	for (int i = 0; i < MAXIMUM_NUMBER_TRACKS; i++) {
		const TRACK_DATA *data = &toc->TrackData[i];
		if (data->TrackNumber == 0 && track_address(data) == 0)
			break;
		if (data->TrackNumber == 0xAA) {
			leadout = track_address(data);
			continue;
		}
		if (data->TrackNumber == track) {
			*first = track_address(data);
			*audio = (data->Control & 4) == 0;
			found = 1;
		} else if (found && leadout == 0 && data->TrackNumber > track) {
			leadout = track_address(data);
		}
	}
	if (!found)
		return -1;
	*last = leadout;
	return 0;
}

static const char *url_device(const char *disc_id)
{
	if (disc_id && disc_id[0] &&
			(disc_id[1] == ':' || (disc_id[0] == '\\' && disc_id[1] == '\\')))
		return disc_id;
	return cdda_device;
}

static int cdda_open(struct input_plugin_data *ip_data)
{
	struct cdda_private *priv;
	char *disc_id = NULL;
	const char *device;
	CDROM_TOC toc;
	int track = -1, audio = 0;
	memset(&toc, 0, sizeof(toc));
	DWORD first, last;
	int rc = 0;

	if (!parse_cdda_url(ip_data->filename, &disc_id, &track, NULL) || track == -1) {
		free(disc_id);
		return -IP_ERROR_INVALID_URI;
	}
	device = url_device(disc_id);
	priv = xnew(struct cdda_private, 1);
	memset(priv, 0, sizeof(*priv));
	priv->track = track;
	priv->used = CDDA_FRAME;
	priv->device = xstrdup(device);
	priv->disc_id = xstrdup(disc_id ? disc_id : "");

	if (open_drive(device, &priv->handle)) {
		rc = -IP_ERROR_ERRNO;
		goto error;
	}
	if (read_toc(priv->handle, &toc)) {
		rc = -IP_ERROR_NO_DISC;
		goto error;
	}
	if (find_track(&toc, track, &audio, &first, &last)) {
		rc = -IP_ERROR_INVALID_URI;
		goto error;
	}
	if (!audio) {
		rc = -IP_ERROR_UNSUPPORTED_FILE_TYPE;
		goto error;
	}
	if (last <= first) {
		rc = -IP_ERROR_NO_DISC;
		goto error;
	}
	priv->first_sector = first;
	priv->last_sector = last;
	priv->current_sector = first;
	ip_data->private = priv;
	ip_data->sf = sf_bits(16) | sf_rate(CDDA_RATE) | sf_channels(2) | sf_signed(1);
	ip_data->sf |= sf_host_endian();
	free(disc_id);
	return 0;
error:
	{
		int save = errno;
		if (priv->handle && priv->handle != INVALID_HANDLE_VALUE)
			CloseHandle(priv->handle);
		free(priv->device);
		free(priv->disc_id);
		free(priv);
		free(disc_id);
		errno = save;
	}
	return rc;
}

static int cdda_close(struct input_plugin_data *ip_data)
{
	struct cdda_private *priv = ip_data->private;
	if (priv) {
		if (priv->handle && priv->handle != INVALID_HANDLE_VALUE)
			CloseHandle(priv->handle);
		free(priv->device);
		free(priv->disc_id);
		free(priv);
		ip_data->private = NULL;
	}
	ip_data->fd = -1;
	return 0;
}

static int read_frame(struct cdda_private *priv)
{
	RAW_READ_INFO info;
	DWORD used = 0;
	memset(&info, 0, sizeof(info));
	info.DiskOffset.QuadPart = (LONGLONG)priv->current_sector * 2048;
	info.SectorCount = 1;
	info.TrackMode = CDDA;
	if (!DeviceIoControl(priv->handle, IOCTL_CDROM_RAW_READ, &info, sizeof(info),
				priv->frame, CDDA_FRAME, &used, NULL)) {
		errno = EIO;
		return -1;
	}
	if (used != CDDA_FRAME) {
		errno = EIO;
		return -1;
	}
	priv->current_sector++;
	priv->used = 0;
	return 0;
}

static int cdda_read(struct input_plugin_data *ip_data, char *buffer, int count)
{
	struct cdda_private *priv = ip_data->private;
	int taken = 0;
	if (!priv || count <= 0)
		return 0;
	while (taken < count) {
		unsigned int left;
		int chunk;
		if (priv->used >= CDDA_FRAME) {
			if (priv->current_sector >= priv->last_sector)
				break;
			if (read_frame(priv))
				return taken ? taken : -IP_ERROR_ERRNO;
		}
		left = CDDA_FRAME - priv->used;
		chunk = count - taken;
		if ((unsigned int)chunk > left)
			chunk = (int)left;
		memcpy(buffer + taken, priv->frame + priv->used, (size_t)chunk);
		priv->used += (unsigned int)chunk;
		taken += chunk;
	}
	return taken;
}

static int cdda_seek(struct input_plugin_data *ip_data, double offset)
{
	struct cdda_private *priv = ip_data->private;
	double frames = offset * CDDA_FRAMES_PER_SEC;
	DWORD target;
	if (!priv || offset < 0)
		return -1;
	target = priv->first_sector + (DWORD)(frames + 0.5);
	if (target >= priv->last_sector)
		return -1;
	priv->current_sector = target;
	priv->used = CDDA_FRAME;
	return 0;
}

static int cdda_read_comments(struct input_plugin_data *ip_data, struct keyval **comments)
{
	struct cdda_private *priv = ip_data->private;
	GROWING_KEYVALS(c);
	char buf[16];
	snprintf(buf, sizeof(buf), "%02d", priv->track);
	comments_add_const(&c, "tracknumber", buf);
	keyvals_terminate(&c);
	*comments = c.keyvals;
	return 0;
}

static int cdda_duration(struct input_plugin_data *ip_data)
{
	struct cdda_private *priv = ip_data->private;
	return (int)((priv->last_sector - priv->first_sector) / CDDA_FRAMES_PER_SEC);
}

static long cdda_bitrate(struct input_plugin_data *ip_data)
{
	return CDDA_RATE * 16 * 2;
}

static char *cdda_codec(struct input_plugin_data *ip_data)
{
	return xstrdup("cdda");
}

static char *cdda_codec_profile(struct input_plugin_data *ip_data)
{
	return xstrdup("CD-DA");
}

const struct input_plugin_ops ip_ops = {
	.open = cdda_open,
	.close = cdda_close,
	.read = cdda_read,
	.seek = cdda_seek,
	.read_comments = cdda_read_comments,
	.duration = cdda_duration,
	.bitrate = cdda_bitrate,
	.codec = cdda_codec,
	.codec_profile = cdda_codec_profile,
};

const struct input_plugin_opt ip_options[] = {
	{ NULL },
};

const int ip_priority = 50;
const char * const ip_extensions[] = { NULL };
const char * const ip_mime_types[] = { "x-content/audio-cdda", NULL };
const unsigned ip_abi_version = IP_ABI_VERSION;

/* Default device and disc id helpers used by shared URL completion code. */
int win_cdda_default_device(char *buffer, size_t size)
{
	WCHAR drives[512];
	DWORD length = GetLogicalDriveStringsW(512, drives);
	for (DWORD i = 0; i < length; ) {
		WCHAR *drive = drives + i;
		size_t n = wcslen(drive);
		if (GetDriveTypeW(drive) == DRIVE_CDROM) {
			char utf8[16];
			int bytes = WideCharToMultiByte(CP_UTF8, 0, drive, (int)n,
					utf8, sizeof(utf8) - 1, NULL, NULL);
			if (bytes > 0) {
				utf8[bytes] = 0;
				if (n > 1 && utf8[bytes - 1] == '\\')
					utf8[bytes - 1] = 0;
				strscpy(buffer, utf8, size);
				return 0;
			}
		}
		i += (DWORD)n + 1;
	}
	strscpy(buffer, "D:", size);
	return 0;
}

int win_cdda_disc_info(const char *device, char **disc_id, int *num_tracks)
{
	HANDLE handle;
	CDROM_TOC toc;
	DWORD leadout = 0, sum = 0;
	unsigned int tracks = 0;
	memset(&toc, 0, sizeof(toc));

	if (open_drive(device, &handle))
		return 0;
	if (read_toc(handle, &toc)) {
		CloseHandle(handle);
		return 0;
	}
	for (int i = 0; i < MAXIMUM_NUMBER_TRACKS; i++) {
		const TRACK_DATA *data = &toc.TrackData[i];
		if (data->TrackNumber == 0 && track_address(data) == 0)
			break;
		if (data->TrackNumber == 0xAA) {
			leadout = track_address(data);
			continue;
		}
		if (data->TrackNumber < toc.FirstTrack || data->TrackNumber > toc.LastTrack)
			continue;
		sum += track_address(data) + 150;
		tracks++;
	}
	CloseHandle(handle);
	if (!tracks || !leadout)
		return 0;
	if (num_tracks)
		*num_tracks = (int)tracks;
	if (disc_id) {
		unsigned int total = leadout + 150;
		unsigned int id = ((sum % 255) << 24) | (total << 8) | tracks;
		char *value = xnew(char, 9);
		snprintf(value, 9, "%08x", id);
		*disc_id = value;
	}
	return 1;
}
