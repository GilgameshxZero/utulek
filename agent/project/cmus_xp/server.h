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

#ifndef CMUS_SERVER_H
#define CMUS_SERVER_H

#include "list.h"
#include "gbuf.h"
#include <sys/types.h>

struct client {
	struct list_head node;
	int fd;
	unsigned int authenticated : 1;
	unsigned int closing : 1;
	unsigned int failed : 1;
	unsigned int pending_commands : 1;
	struct gbuf input, output;
	size_t output_position;
};

extern int server_socket;
extern struct list_head client_head;

void server_init(char *address);
void server_exit(void);
void server_accept(void);
void server_serve(struct client *client);
ssize_t server_write(int fd, const void *buffer, size_t count);
void server_poll(void);

#endif
