/*
 * Copyright 2008-2013 Various Authors
 * Copyright 2005 Timo Hirvonen
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

#include "file.h"
#include "xmalloc.h"

#include <sys/types.h>
#include <sys/stat.h>
#include <sys/mman.h>
#include <fcntl.h>
#include <unistd.h>
#include <stdlib.h>
#include <errno.h>
#include <limits.h>
#include <stdint.h>
#include <stdatomic.h>
#include <stdio.h>
#include <string.h>

ssize_t read_all(int fd, void *buf, size_t count)
{
	char *buffer = buf;
	ssize_t pos = 0;

	do {
		ssize_t rc;

		rc = read(fd, buffer + pos, count - pos);
		if (rc == -1) {
			if (errno == EINTR || errno == EAGAIN)
				continue;
			return -1;
		}
		if (rc == 0) {
			/* eof */
			break;
		}
		pos += rc;
	} while (count - pos > 0);
	return pos;
}

ssize_t write_all(int fd, const void *buf, size_t count)
{
	const char *buffer = buf;
	if (count > PTRDIFF_MAX) { errno = EINVAL; return -1; }
	ssize_t count_save = count;

	do {
		int rc;

		rc = write(fd, buffer, count > INT_MAX ? INT_MAX : count);
		if (rc == -1) {
			if (errno == EINTR || errno == EAGAIN)
				continue;
			return -1;
		}
		if (!rc && count) { errno = EIO; return -1; }
		buffer += rc;
		count -= rc;
	} while (count > 0);
	return count_save;
}

static char *save_target(const char *filename)
{
#ifdef CMUS_NATIVE_WINDOWS
	wchar_t *wide = win_path(filename);
	if (!wide) return NULL;
	HANDLE handle = CreateFileW(wide, FILE_READ_ATTRIBUTES,
		FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE, NULL, OPEN_EXISTING, 0, NULL);
	if (handle == INVALID_HANDLE_VALUE) {
		DWORD error = GetLastError(), attrs = GetFileAttributesW(wide);
		free(wide);
		if ((error == ERROR_FILE_NOT_FOUND || error == ERROR_PATH_NOT_FOUND) &&
				(attrs == INVALID_FILE_ATTRIBUTES || !(attrs & FILE_ATTRIBUTE_REPARSE_POINT)))
			return xstrdup(filename);
		errno = EACCES; return NULL;
	}
	free(wide);
	DWORD size = GetFinalPathNameByHandleW(handle, NULL, 0, FILE_NAME_NORMALIZED);
	wide = size ? malloc((size + 1) * sizeof(*wide)) : NULL;
	DWORD length = wide ? GetFinalPathNameByHandleW(handle, wide, size + 1, FILE_NAME_NORMALIZED) : 0;
	CloseHandle(handle);
	if (!length || length > size) { free(wide); errno = EIO; return NULL; }
	char *plain = win_normalize(win_utf8(wide));
	char *target = plain ? win_absolute_path(plain, NULL) : NULL;
	free(plain);
	free(wide);
	return target;
#else
	char *target = realpath(filename, NULL);
	if (target) return target;
	if (errno != ENOENT) return NULL;
	struct stat st;
	/* Preserve dangling links rather than replacing them with a regular file. */
	if (!lstat(filename, &st) && S_ISLNK(st.st_mode)) { errno = ENOENT; return NULL; }
	return xstrdup(filename);
#endif
}

int file_write_atomic(const char *filename, int (*write_cb)(int, void *), void *data)
{
	static _Atomic unsigned int sequence;
	char *target = save_target(filename);
	if (!target) return -1;
	struct stat st;
	int exists = stat(target, &st) == 0;
	if (exists && !S_ISREG(st.st_mode)) { free(target); errno = EINVAL; return -1; }
	char *temporary = xmalloc(strlen(target) + 64);
	int fd = -1;
	for (int attempt = 0; attempt < 64; attempt++) {
		snprintf(temporary, strlen(target) + 64, "%s.tmp.%lu.%u", target,
			(unsigned long)getpid(), atomic_fetch_add(&sequence, 1));
		fd = open(temporary, O_WRONLY | O_CREAT | O_EXCL, 0666);
		if (fd >= 0 || errno != EEXIST) break;
	}
	int rc = -1, saved_errno = errno;
	if (fd >= 0) {
#ifndef CMUS_NATIVE_WINDOWS
		if (exists && fchmod(fd, st.st_mode & 0777)) goto close_temporary;
#endif
		rc = write_cb(fd, data);
		if (!rc) rc = fsync(fd);
close_temporary:
		saved_errno = errno;
		if (close(fd) && !rc) { rc = -1; saved_errno = errno; }
		if (!rc) { rc = rename(temporary, target); saved_errno = errno; }
		if (rc) unlink(temporary);
	}
	free(temporary); free(target);
	errno = saved_errno;
	return rc;
}

char *mmap_file(const char *filename, ssize_t *size)
{
	struct stat st;
	char *buf;
	int fd;

	fd = open(filename, O_RDONLY);
	if (fd == -1)
		goto err;

	if (fstat(fd, &st) == -1)
		goto close_err;

	/* can't mmap empty files */
	buf = NULL;
	if (st.st_size) {
		buf = mmap(NULL, st.st_size, PROT_READ, MAP_PRIVATE, fd, 0);
		if (buf == MAP_FAILED)
			goto close_err;
	}

	close(fd);
	*size = st.st_size;
	return buf;

close_err:
	close(fd);
err:
	*size = -1;
	return NULL;
}

void buffer_for_each_line(const char *buf, int size,
		int (*cb)(void *data, const char *line),
		void *data)
{
	char *line = NULL;
	int line_size = 0, pos = 0;

	while (pos < size) {
		int end, len;

		end = pos;
		while (end < size && buf[end] != '\n')
			end++;

		len = end - pos;
		if (end > pos && buf[end - 1] == '\r')
			len--;

		if (len >= line_size) {
			line_size = len + 1;
			line = xrenew(char, line, line_size);
		}
		memcpy(line, buf + pos, len);
		line[len] = 0;
		pos = end + 1;

		if (cb(data, line))
			break;
	}
	free(line);
}

void buffer_for_each_line_reverse(const char *buf, int size,
		int (*cb)(void *data, const char *line),
		void *data)
{
	char *line = NULL;
	int line_size = 0, end = size - 1;

	while (end >= 0) {
		int pos, len;

		if (end > 1 && buf[end] == '\n' && buf[end - 1] == '\r')
			end--;

		pos = end;
		while (pos > 0 && buf[pos - 1] != '\n')
			pos--;

		len = end - pos;
		if (len >= line_size) {
			line_size = len + 1;
			line = xrenew(char, line, line_size);
		}
		memcpy(line, buf + pos, len);
		line[len] = 0;
		end = pos - 1;

		if (cb(data, line))
			break;
	}
	free(line);
}

int file_for_each_line(const char *filename,
		int (*cb)(void *data, const char *line),
		void *data)
{
	char *buf;
	ssize_t size;

#ifdef CMUS_NATIVE_WINDOWS
    const char *resource;
    size_t resource_size;
    if (win_resource(filename, &resource, &resource_size)) {
        buffer_for_each_line(resource, resource_size, cb, data);
        return 0;
    }
#endif
    buf = mmap_file(filename, &size);
	if (size == -1)
		return -1;

	if (buf) {
		buffer_for_each_line(buf, size, cb, data);
		munmap(buf, size);
	}
	return 0;
}
