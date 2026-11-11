/* Minimal iconv for the native Windows build using Windows NLS code pages.
 * Covers metadata encodings used by audio tags and terminal conversion.
 * GPL-2.0-or-later. */
#include "compat.h"
#include <iconv.h>
#include <errno.h>
#include <stdlib.h>
#include <string.h>

struct win_iconv {
	UINT from;
	UINT to;
};

static void normalize(const char *name, char *out, size_t size)
{
	size_t used = 0;
	for (; *name && used + 1 < size; name++) {
		unsigned char c = (unsigned char)*name;
		if (c == '-' || c == '_' || c == ' ' || c == '.')
			continue;
		out[used++] = (char)(c >= 'A' && c <= 'Z' ? c - 'A' + 'a' : c);
	}
	out[used] = 0;
}

static int numeric_codepage(const char *name, UINT *page)
{
	if (name[0] != 'c' || name[1] != 'p' || !name[2])
		return 0;
	char *end;
	unsigned long value = strtoul(name + 2, &end, 10);
	if (*end || value > 65535 || !IsValidCodePage(value))
		return 0;
	*page = (UINT)value;
	return 1;
}

/* Returns 0 when the encoding is unknown. */
static UINT codepage(const char *name)
{
	char n[64];
	normalize(name, n, sizeof(n));
	if (!strcmp(n, "utf8") || !strcmp(n, "utf"))
		return CP_UTF8;
	if (!strcmp(n, "utf16") || !strcmp(n, "utf16le") || !strcmp(n, "ucs2") ||
			!strcmp(n, "ucs2le") || !strcmp(n, "unicode"))
		return 1200;
	if (!strcmp(n, "utf16be") || !strcmp(n, "ucs2be"))
		return 1201;
	if (!strcmp(n, "ascii") || !strcmp(n, "usascii") || !strcmp(n, "ansi_x3.41968") ||
			!strcmp(n, "iso646us"))
		return 20127;
	if (!strcmp(n, "ansi") || !strcmp(n, "locale") || !strcmp(n, "char"))
		return CP_ACP;
	if (!strcmp(n, "latin1") || !strcmp(n, "iso88591"))
		return 28591;
	if (!strcmp(n, "latin2") || !strcmp(n, "iso88592"))
		return 28592;
	if (!strcmp(n, "latin3") || !strcmp(n, "iso88593"))
		return 28593;
	if (!strcmp(n, "latin4") || !strcmp(n, "iso88594"))
		return 28594;
	if (!strcmp(n, "cyrillic") || !strcmp(n, "iso88595"))
		return 28595;
	if (!strcmp(n, "arabic") || !strcmp(n, "iso88596"))
		return 28596;
	if (!strcmp(n, "greek") || !strcmp(n, "iso88597"))
		return 28597;
	if (!strcmp(n, "hebrew") || !strcmp(n, "iso88598"))
		return 28598;
	if (!strcmp(n, "latin5") || !strcmp(n, "iso88599"))
		return 28599;
	if (!strcmp(n, "latin6") || !strcmp(n, "iso885910"))
		return 28600;
	if (!strcmp(n, "iso885911") || !strcmp(n, "tis620"))
		return 28601;
	if (!strcmp(n, "iso885913") || !strcmp(n, "latin7"))
		return 28603;
	if (!strcmp(n, "iso885914"))
		return 28604;
	if (!strcmp(n, "iso885915") || !strcmp(n, "latin9"))
		return 28605;
	if (!strcmp(n, "iso885916"))
		return 28606;
	if (!strcmp(n, "koi8r"))
		return 20866;
	if (!strcmp(n, "koi8u"))
		return 21866;
	if (!strcmp(n, "shiftjis") || !strcmp(n, "sjis") || !strcmp(n, "windows31j") ||
			!strcmp(n, "cp932"))
		return 932;
	if (!strcmp(n, "eucjp") || !strcmp(n, "ujis"))
		return 51932;
	if (!strcmp(n, "iso2022jp"))
		return 50220;
	if (!strcmp(n, "euckr") || !strcmp(n, "uhc"))
		return 949;
	if (!strcmp(n, "gbk") || !strcmp(n, "gb2312") || !strcmp(n, "eucn"))
		return 936;
	if (!strcmp(n, "gb18030"))
		return 54936;
	if (!strcmp(n, "big5") || !strcmp(n, "cp950") || !strcmp(n, "big5hkscs"))
		return 950;
	if (!strcmp(n, "utf7"))
		return 65000;
	if (!strcmp(n, "hz"))
		return 52936;
	if (!strcmp(n, "mac") || !strcmp(n, "macintosh"))
		return 10000;
	if (!strncmp(n, "windows", 7)) {
		unsigned long value = strtoul(n + 7, NULL, 10);
		if (value >= 1250 && value <= 1258)
			return (UINT)value;
	}
	if (!strncmp(n, "cp", 2)) {
		UINT page;
		if (numeric_codepage(n, &page))
			return page;
	}
	if (!strncmp(n, "iso8859", 7)) {
		unsigned long value = strtoul(n + 7, NULL, 10);
		if (value >= 1 && value <= 16 && value != 12)
			return (UINT)(28590 + value);
	}
	return 0;
}

iconv_t iconv_open(const char *tocode, const char *fromcode)
{
	UINT from = codepage(fromcode), to = codepage(tocode);
	struct win_iconv *cd;
	if (!from || !to) {
		errno = EINVAL;
		return (iconv_t)-1;
	}
	cd = malloc(sizeof(*cd));
	if (!cd) {
		errno = ENOMEM;
		return (iconv_t)-1;
	}
	cd->from = from;
	cd->to = to;
	return (iconv_t)cd;
}

int iconv_close(iconv_t handle)
{
	free(handle);
	return 0;
}

size_t iconv(iconv_t handle, char **inbuf, size_t *inbytesleft,
		char **outbuf, size_t *outbytesleft)
{
	struct win_iconv *cd = handle;
	wchar_t *wide = NULL;
	char *out = NULL;
	size_t result = (size_t)-1;
	int wide_len, out_len;
	BOOL used_default = FALSE;

	if (!cd || !inbuf || !*inbuf || !inbytesleft || !*inbytesleft)
		return 0;
	if (!outbuf || !*outbuf || !outbytesleft) {
		errno = E2BIG;
		return (size_t)-1;
	}

	if (cd->from == 1200 || cd->from == 1201) {
		if (*inbytesleft & 1) {
			errno = EINVAL;
			return (size_t)-1;
		}
		wide_len = (int)(*inbytesleft / 2);
		wide = malloc((size_t)wide_len * sizeof(*wide));
		if (!wide) {
			errno = ENOMEM;
			goto done;
		}
		if (cd->from == 1200) {
			memcpy(wide, *inbuf, (size_t)wide_len * sizeof(*wide));
		} else {
			for (int i = 0; i < wide_len; i++) {
				const unsigned char *p = (const unsigned char *)*inbuf + i * 2;
				wide[i] = (wchar_t)((p[0] << 8) | p[1]);
			}
		}
	} else {
		wide_len = MultiByteToWideChar(cd->from, 0, *inbuf, (int)*inbytesleft, NULL, 0);
		if (wide_len <= 0) {
			errno = EILSEQ;
			goto done;
		}
		wide = malloc((size_t)wide_len * sizeof(*wide));
		if (!wide) {
			errno = ENOMEM;
			goto done;
		}
		if (!MultiByteToWideChar(cd->from, 0, *inbuf, (int)*inbytesleft,
					wide, wide_len)) {
			errno = EILSEQ;
			goto done;
		}
	}

	if (cd->to == 1200 || cd->to == 1201) {
		out_len = wide_len * 2;
		if ((size_t)out_len > *outbytesleft) {
			errno = E2BIG;
			goto done;
		}
		if (cd->to == 1200) {
			memcpy(*outbuf, wide, (size_t)out_len);
		} else {
			for (int i = 0; i < wide_len; i++) {
				unsigned char *p = (unsigned char *)*outbuf + i * 2;
				p[0] = (unsigned char)(wide[i] >> 8);
				p[1] = (unsigned char)(wide[i] & 255);
			}
		}
		out = *outbuf;
	} else {
		out_len = WideCharToMultiByte(cd->to, 0, wide, wide_len, NULL, 0,
				NULL, &used_default);
		if (out_len <= 0) {
			errno = EILSEQ;
			goto done;
		}
		if ((size_t)out_len > *outbytesleft) {
			errno = E2BIG;
			goto done;
		}
		if (!WideCharToMultiByte(cd->to, 0, wide, wide_len, *outbuf, (int)*outbytesleft,
					NULL, &used_default)) {
			errno = EILSEQ;
			goto done;
		}
		out = *outbuf + out_len;
	}

	*inbuf += *inbytesleft;
	*inbytesleft = 0;
	*outbytesleft -= (size_t)out_len;
	if (cd->to == 1200 || cd->to == 1201)
		*outbuf = out + out_len;
	else
		*outbuf = out;
	result = 0;
done:
	free(wide);
	return result;
}
