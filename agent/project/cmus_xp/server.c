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

#include "server.h"
#include "prog.h"
#include "command_mode.h"
#include "search_mode.h"
#include "options.h"
#include "output.h"
#include "utils.h"
#include "xmalloc.h"
#include "player.h"
#include "file.h"
#include "compiler.h"
#include "debug.h"
#include "gbuf.h"
#include "ui_curses.h"
#include "misc.h"
#include "keyval.h"
#include "convert.h"
#include "format_print.h"

#include <stdarg.h>
#include <unistd.h>
#include <sys/types.h>
#ifndef CMUS_NATIVE_WINDOWS
#include <sys/socket.h>
#include <sys/un.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <netdb.h>
#else
#include "windows/remote_io.h"
#endif
#include <errno.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <ctype.h>

int server_socket;
LIST_HEAD(client_head);

static char *title_buf = NULL;

static int server_is_local;
static unsigned client_count;

#ifndef CMUS_NATIVE_WINDOWS
static union {
	struct sockaddr sa;
	struct sockaddr_un un;
	struct sockaddr_storage sas;
} addr;
#endif

#define MAX_CLIENTS 10

#define MAX_COMMAND (1024 * 1024)
#define MAX_REPLY (32 * 1024 * 1024)
#ifdef CMUS_NATIVE_WINDOWS
#define connection_read win_remote_read
#define connection_write win_remote_write
#define connection_close win_remote_close
#else
#define connection_read read
#define connection_write write
#define connection_close close
#endif

ssize_t server_write(int fd, const void *buffer, size_t count)
{
    struct client *client;
    list_for_each_entry(client, &client_head, node) {
        if (client->fd != fd) continue;
        if (client->failed || count > MAX_REPLY - client->output.len) {
            client->failed = 1; errno = EOVERFLOW; return -1;
        }
        gbuf_add_bytes(&client->output, buffer, count);
        return count;
    }
    errno = EBADF;
    return -1;
}

static void close_client(struct client *client)
{
    connection_close(client->fd);
    list_del(&client->node);
    gbuf_free(&client->input);
    gbuf_free(&client->output);
    free(client);
    client_count--;
}

static void flush_client(struct client *client)
{
    size_t left = client->output.len - client->output_position;
    if (!left) return;
    if (left > 64 * 1024) left = 64 * 1024;
    ssize_t count = connection_write(client->fd, client->output.buffer + client->output_position, left);
    if (count < 0 && (errno == EAGAIN || errno == EINTR)) return;
    if (count <= 0) { client->failed = 1; return; }
    client->output_position += count;
    if (client->output_position == client->output.len) {
        gbuf_clear(&client->output);
        client->output_position = 0;
    }
}

static int cmd_status(struct client *client)
{
	const char *export_options[] = {
		"aaa_mode",
		"continue",
		"play_library",
		"play_sorted",
		"replaygain",
		"replaygain_limit",
		"replaygain_preamp",
		"repeat",
		"repeat_current",
		"shuffle",
		"softvol",
		NULL
	};
	const struct track_info *ti;
	struct cmus_opt *opt;
	char optbuf[OPTION_MAX_SIZE];
	GBUF(buf);
	int vol_left, vol_right;
	int i, ret;
	enum player_status status;

	gbuf_addf(&buf, "status %s\n", player_status_names[player_info.status]);
	if (player_info.output_sf) {
		gbuf_addf(&buf, "output_sample_rate %u\n", sf_get_rate(player_info.output_sf));
		gbuf_addf(&buf, "output_bits %u\n", sf_get_bits(player_info.output_sf));
		gbuf_addf(&buf, "output_channels %u\n", sf_get_channels(player_info.output_sf));
	}
	ti = player_info.ti;
	if (ti) {
		gbuf_addf(&buf, "file %s\n", escape(ti->filename));
		gbuf_addf(&buf, "duration %d\n", ti->duration);
		gbuf_addf(&buf, "position %d\n", player_info.pos);
		for (i = 0; ti->comments[i].key; i++)
			gbuf_addf(&buf, "tag %s %s\n",
					ti->comments[i].key,
					escape(ti->comments[i].val));
	}

	/* add track metadata to cmus-status */
	status = player_info.status;
	if (status == PLAYER_STATUS_PLAYING && ti && is_http_url(player_info.ti->filename)) {
	const char *title = get_stream_title();
		if (title != NULL) {
			free(title_buf);
			title_buf = to_utf8(title, icecast_default_charset);
			// we have a stream title (probably artist/track/album info)
			gbuf_addf(&buf, "stream %s\n", escape(title_buf));
		} else if (ti->comment != NULL) {
			// fallback to the radio station name
			gbuf_addf(&buf, "stream %s\n", escape(ti->comment));
		}
	}

	/* output options */
	for (i = 0; export_options[i]; i++) {
		opt = option_find(export_options[i]);
		if (opt) {
			opt->get(opt->data, optbuf, OPTION_MAX_SIZE);
			gbuf_addf(&buf, "set %s %s\n", opt->name, optbuf);
		}
	}

	/* get volume (copied from ui_curses.c) */
	if (soft_vol) {
		vol_left = soft_vol_l;
		vol_right = soft_vol_r;
	} else if (!volume_max) {
		vol_left = vol_right = -1;
	} else {
		vol_left = scale_to_percentage(volume_l, volume_max);
		vol_right = scale_to_percentage(volume_r, volume_max);
	}

	/* output volume */
	gbuf_addf(&buf, "set vol_left %d\n", vol_left);
	gbuf_addf(&buf, "set vol_right %d\n", vol_right);

	gbuf_add_str(&buf, "\n");

	ret = server_write(client->fd, buf.buffer, buf.len);
	gbuf_free(&buf);
	return ret;
}

static int cmd_format_print(struct client *client, char *arg)
{
	if (run_only_safe_commands) {
		d_print("trying to execute unsafe command over net\n");
		return server_write(client->fd, "\n", strlen("\n"));
	}

	int args_idx, ac, i, ret;
	char **args = NULL;

	if (arg)
		args = parse_cmd(arg, &args_idx, &ac);

	if (args == NULL) {
		error_msg("not enough arguments\n");
		return server_write(client->fd, "\n", strlen("\n"));
	}

	GBUF(buf);

	const struct format_option *fopts = get_global_fopts();
	for (i = 0; i < ac; ++i) {
		if (format_valid(args[i], fopts))
			format_print(&buf, 0, args[i], fopts);
		gbuf_add_ch(&buf, '\n');
		free(args[i]);
	}
	gbuf_add_ch(&buf, '\n');

	ret = server_write(client->fd, buf.buffer, buf.len);
	gbuf_free(&buf);
	free(args);
	return ret;
}

static ssize_t send_answer(int fd, const char *format, ...)
{
	char buf[512];
	va_list ap;

	va_start(ap, format);
	vsnprintf(buf, sizeof(buf), format, ap);
	va_end(ap);

	return server_write(fd, buf, strlen(buf));
}

static void read_commands(struct client *client)
{
    char incoming[8192];
    ssize_t rc = client->closing ? -1 : connection_read(client->fd, incoming, sizeof(incoming));
    if (client->closing) errno = EAGAIN;
    if (rc < 0 && errno != EAGAIN && errno != EINTR) { client->failed = 1; return; }
    if (rc == 0) client->closing = 1;
    if (rc > 0) {
        if (client->input.len + rc > MAX_COMMAND) { client->failed = 1; return; }
        gbuf_add_bytes(&client->input, incoming, rc);
    }
    char *buf = client->input.buffer;
    size_t pos = client->input.len, s = 0, i;
    unsigned command_count = 0;
    if (!client->authenticated) client->authenticated = server_is_local;
		s = 0;
		for (i = 0; i < pos && command_count < 64; i++) {
			const char *line, *msg;
			char *cmd, *arg;
			int ret;

			if (buf[i] != '\n')
				continue;

			command_count++;
			buf[i] = 0;
			line = buf + s;
			s = i + 1;

			if (!client->authenticated) {
				if (!server_password) {
					msg = "password is unset, tcp/ip disabled";
					d_print("%s\n", msg);
					ret = send_answer(client->fd, "%s\n\n", msg);
					client->closing = 1;
                    goto done;
				}
				if (strncmp(line, "passwd ", 7) == 0)
					line += 7;
				client->authenticated = !strcmp(line, server_password);
				if (!client->authenticated) {
					msg = "authentication failed";
					d_print("%s\n", msg);
					ret = send_answer(client->fd, "%s\n\n", msg);
					client->closing = 1;
                    goto done;
				}
				ret = server_write(client->fd, "\n", 1);
				continue;
			}

			while (isspace((unsigned char)*line))
				line++;

			if (*line == '/') {
				int restricted = 0;
				line++;
				search_direction = SEARCH_FORWARD;
				if (*line == '/') {
					line++;
					restricted = 1;
				}
				search_text(line, restricted, 1);
				ret = server_write(client->fd, "\n", 1);
			} else if (*line == '?') {
				int restricted = 0;
				line++;
				search_direction = SEARCH_BACKWARD;
				if (*line == '?') {
					line++;
					restricted = 1;
				}
				search_text(line, restricted, 1);
				ret = server_write(client->fd, "\n", 1);
			} else if (parse_command(line, &cmd, &arg)) {
				if (!strcmp(cmd, "status")) {
					ret = cmd_status(client);
				} else if (!strcmp(cmd, "format_print")) {
					ret = cmd_format_print(client, arg);
				} else {
					if (strcmp(cmd, "passwd") != 0) {
						set_client_fd(client->fd);
						run_parsed_command(cmd, arg);
						set_client_fd(-1);
					}
					ret = server_write(client->fd, "\n", 1);
				}
				free(cmd);
				free(arg);
			} else {
				// don't hang cmus-remote
				ret = server_write(client->fd, "\n", 1);
			}
			if (ret < 0) {
				d_print("write: %s\n", strerror(errno));
				client->failed = 1;
                goto done;
			}
		}
done:
    memmove(buf, buf + s, pos - s);
    client->input.len = pos - s;
    buf[client->input.len] = 0;
    client->pending_commands = memchr(buf, '\n', client->input.len) != NULL;
}

void server_accept(void)
{
#ifdef CMUS_NATIVE_WINDOWS
    int fd = win_remote_accept(server_socket);
#else
    int fd = accept(server_socket, NULL, NULL);
#endif
    if (fd < 0) return;
    if (client_count == MAX_CLIENTS) { connection_close(fd); return; }
#ifndef CMUS_NATIVE_WINDOWS
    if (fcntl(fd, F_SETFL, O_NONBLOCK) < 0) { close(fd); return; }
#endif
    struct client *client = xnew0(struct client, 1);
    client->fd = fd;
    client->input.buffer = client->output.buffer = gbuf_empty_buffer;
    list_add_tail(&client->node, &client_head);
    client_count++;
}

void server_serve(struct client *client)
{
    run_only_safe_commands = !server_is_local;
    if ((!client->closing || client->pending_commands) && !client->failed) read_commands(client);
    run_only_safe_commands = 0;
    flush_client(client);
    if (client->failed || (client->closing && !client->output.len && !client->pending_commands)) close_client(client);
}

void server_poll(void)
{
#ifdef CMUS_NATIVE_WINDOWS
    if (win_remote_ready(server_socket, 0)) server_accept();
    struct list_head *item = client_head.next;
    while (item != &client_head) {
        struct list_head *next = item->next;
        struct client *client = container_of(item, struct client, node);
        if (client->pending_commands || win_remote_ready(client->fd, 0) ||
                (client->output.len && win_remote_ready(client->fd, 1))) server_serve(client);
        item = next;
    }
#endif
}

void server_init(char *address)
{
#ifdef CMUS_NATIVE_WINDOWS
    server_is_local = win_remote_local(address);
    server_socket = win_remote_listen(address);
    if (server_socket < 0) die_errno("remote listen");
#else
	const char *port = STRINGIZE(DEFAULT_PORT);
	size_t addrlen;

	server_is_local = strchr(address, '/') != NULL;
	if (server_is_local) {
		addr.sa.sa_family = AF_UNIX;
		strncpy(addr.un.sun_path, address, sizeof(addr.un.sun_path) - 1);

		addrlen = sizeof(struct sockaddr_un);
	} else {
		const struct addrinfo hints = {
			.ai_socktype = SOCK_STREAM
		};
		struct addrinfo *result;
		char *s = strrchr(address, ':');
		int rc;

		if (s) {
			*s++ = 0;
			port = s;
		}

		rc = getaddrinfo(address, port, &hints, &result);
		if (rc != 0)
			die("getaddrinfo: %s\n", gai_strerror(rc));
		memcpy(&addr.sa, result->ai_addr, result->ai_addrlen);
		addrlen = result->ai_addrlen;
		freeaddrinfo(result);
	}

	server_socket = socket(addr.sa.sa_family, SOCK_STREAM, 0);
	if (server_socket == -1)
		die_errno("socket");

	if (bind(server_socket, &addr.sa, addrlen) == -1) {
		int sock;

		if (errno != EADDRINUSE)
			die_errno("bind");

		/* address already in use */
		if (addr.sa.sa_family != AF_UNIX)
			die("cmus is already listening on %s:%s\n", address, port);

		/* try to connect to server */
		sock = socket(AF_UNIX, SOCK_STREAM, 0);
		if (sock == -1)
			die_errno("socket");

		if (connect(sock, &addr.sa, addrlen) == -1) {
			if (errno != ENOENT && errno != ECONNREFUSED)
				die_errno("connect");

			/* server not running => dead socket */

			/* try to remove dead socket */
			if (unlink(addr.un.sun_path) == -1 && errno != ENOENT)
				die_errno("unlink");
			if (bind(server_socket, &addr.sa, addrlen) == -1)
				die_errno("bind");
		} else {
			/* server already running */
			die("cmus is already listening on socket %s\n", address);
		}
		close(sock);
	}

	if (listen(server_socket, MAX_CLIENTS) == -1)
		die_errno("listen");
#endif
}

void server_exit(void)
{
    while (!list_empty(&client_head)) {
        struct client *client = container_of(client_head.next, struct client, node);
        flush_client(client);
        close_client(client);
    }
    connection_close(server_socket);
#ifndef CMUS_NATIVE_WINDOWS
    if (addr.sa.sa_family == AF_UNIX) unlink(addr.un.sun_path);
#endif
}
