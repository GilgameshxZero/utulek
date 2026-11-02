/* Features staged after the first local-playback milestone. GPL-2.0-or-later. */
#include "../server.h"
#include "../http.h"
#include "../spawn.h"
#include <errno.h>
#include <process.h>
#include <stdlib.h>
int server_socket = -1;
LIST_HEAD(client_head);
void server_init(char *address) { (void)address; }
void server_exit(void) { }
void server_accept(void) { }
void server_serve(struct client *client) { (void)client; }
int spawn(char *argv[], int *status, int do_wait)
{
    intptr_t rc = _spawnvp(do_wait ? _P_WAIT : _P_NOWAIT, argv[0], (const char *const *)argv);
    if (rc == -1) return -1;
    if (do_wait) { if (status) *status = ((int)rc & 255) << 8; }
    else CloseHandle((HANDLE)rc);
    return 0;
}
int http_parse_uri(const char *uri, struct http_uri *u) { (void)uri; (void)u; errno = ENOTSUP; return -1; }
void http_free_uri(struct http_uri *u) { (void)u; }
int http_open(struct http_get *h, int timeout) { (void)h; (void)timeout; errno = ENOTSUP; return -1; }
int http_get(struct http_get *h, struct keyval *headers, int timeout) { (void)h; (void)headers; (void)timeout; errno = ENOTSUP; return -1; }
void http_get_free(struct http_get *h) { (void)h; }
char *http_read_body(int fd, size_t *size, int timeout) { (void)fd; (void)size; (void)timeout; errno = ENOTSUP; return NULL; }

#include "../xmalloc.h"
#include "../compiler.h"
char *base64_encode(const char *str)
{
	static const char t[64] CMUS_NONSTRING = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
	int str_len, buf_len, i, s, d;
	char *buf;
	unsigned char b0, b1, b2;

	str_len = strlen(str);
	buf_len = (str_len + 2) / 3 * 4 + 1;
	buf = xnew(char, buf_len);
	s = 0;
	d = 0;
	for (i = 0; i < str_len / 3; i++) {
		b0 = str[s++];
		b1 = str[s++];
		b2 = str[s++];

		/* 6 ms bits of b0 */
		buf[d++] = t[b0 >> 2];

		/* 2 ls bits of b0 . 4 ms bits of b1 */
		buf[d++] = t[((b0 << 4) | (b1 >> 4)) & 0x3f];

		/* 4 ls bits of b1 . 2 ms bits of b2 */
		buf[d++] = t[((b1 << 2) | (b2 >> 6)) & 0x3f];

		/* 6 ls bits of b2 */
		buf[d++] = t[b2 & 0x3f];
	}
	switch (str_len % 3) {
	case 2:
		b0 = str[s++];
		b1 = str[s++];

		/* 6 ms bits of b0 */
		buf[d++] = t[b0 >> 2];

		/* 2 ls bits of b0 . 4 ms bits of b1 */
		buf[d++] = t[((b0 << 4) | (b1 >> 4)) & 0x3f];

		/* 4 ls bits of b1 */
		buf[d++] = t[(b1 << 2) & 0x3f];

		buf[d++] = '=';
		break;
	case 1:
		b0 = str[s++];

		/* 6 ms bits of b0 */
		buf[d++] = t[b0 >> 2];

		/* 2 ls bits of b0 */
		buf[d++] = t[(b0 << 4) & 0x3f];

		buf[d++] = '=';
		buf[d++] = '=';
		break;
	case 0:
		break;
	}
	buf[d] = 0;
	return buf;
}
