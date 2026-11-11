/* PDCurses display adapter for Windows terminals and nested ConPTY hosts.
 * Keep curses' screen/input model, but write explicitly positioned VT spans
 * instead of mixing WriteConsoleOutput rectangles and cursor-moving writes.
 * GPL-2.0-or-later.
 */
#include "wincon/pdcwin.h"
#include "common/acsuni.h"
#include "curses_unicode.h"
#include <stdio.h>

DWORD pdc_last_blink;
/* Pair zero and the application's "default" colors inherit the terminal theme.
 * PDCurses 3.9 otherwise normalizes -1 to console palette indexes in init_pair.
 */
static unsigned char pair_defaults[PDC_COLOR_PAIRS] = {3};

void win_curses_pair_defaults(int pair, int foreground, int background)
{
    unsigned char defaults = (foreground == -1) | ((background == -1) << 1);
    if (pair >= 0 && pair < PDC_COLOR_PAIRS && pair_defaults[pair] != defaults) {
        pair_defaults[pair] = defaults;
        if (curscr) curscr->_clear = TRUE;
    }
}

static void write_wide(const WCHAR *text, DWORD length)
{
    while (length) {
        DWORD written;
        if (!WriteConsoleW(pdc_con_out, text, length, &written, NULL) || !written)
            return;
        text += written;
        length -= written;
    }
}

static void write_control(const char *text)
{
    WCHAR wide[128];
    int n = MultiByteToWideChar(CP_UTF8, 0, text, -1, wide, 128);
    if (n > 1) write_wide(wide, n - 1);
}

static void vt_mode(void)
{
    /* Disable wrapping so writing the bottom-right cell cannot scroll. */
    SetConsoleMode(pdc_con_out, ENABLE_PROCESSED_OUTPUT |
        ENABLE_VIRTUAL_TERMINAL_PROCESSING | DISABLE_NEWLINE_AUTO_RETURN);
}

void PDC_gotoyx(int row, int col)
{
    char escape[48];
    vt_mode();
    snprintf(escape, sizeof escape, "\033[%d;%dH", row + 1, col + 1);
    write_control(escape);
}

static void color(char *out, size_t size, short value, int background)
{
    if (value < 0)
        snprintf(out, size, "%d", background ? 49 : 39);
    else if (pdc_color[value].mapped)
        snprintf(out, size, "%d;2;%d;%d;%d", background ? 48 : 38,
            DIVROUND(pdc_color[value].r * 255, 1000),
            DIVROUND(pdc_color[value].g * 255, 1000),
            DIVROUND(pdc_color[value].b * 255, 1000));
    else if (value < 16) {
        int ansi = pdc_curstoansi[value];
        snprintf(out, size, "%d", (background ? 40 : 30) +
            (ansi < 8 ? ansi : ansi - 8 + 60));
    } else
        snprintf(out, size, "%d;5;%d", background ? 48 : 38, value);
}

void PDC_transform_line(int row, int x, int len, const chtype *src)
{
    /* A changed span may begin at the second cell of a wide glyph. Repaint
     * its lead cell too, so the terminal cannot retain half an old glyph. */
    if (len > 0 && x > 0 && (*src & A_CHARTEXT) == CMUS_WIDE_CONTINUATION) {
        src--;
        x--;
        len++;
    }
    while (len > 0) {
        attr_t attr = *src & (A_ATTRIBUTES ^ A_ALTCHARSET);
        short fore, back;
        WCHAR text[512];
        char fg[48], bg[48], escape[128];
        int n = 0, cells = 0;
        int pair = PAIR_NUMBER(attr);
        PDC_pair_content(pair, &fore, &back);
        if (pair_defaults[pair] & 1) fore = -1;
        if (pair_defaults[pair] & 2) back = -1;
        /* Match the wincon backend's bright-color interpretation. */
        if ((attr & A_BLINK) && back >= 0 && back < 8) back |= 8;
        color(fg, sizeof fg, fore, 0);
        color(bg, sizeof bg, back, 1);
        PDC_gotoyx(row, x);
        snprintf(escape, sizeof escape, "\033[0;%s;%s%s%s%s%sm", fg, bg,
            (attr & A_REVERSE) ? ";7" : "",
            (attr & A_BOLD) ? ";1" : "",
            (attr & A_UNDERLINE) ? ";4" : "",
            (attr & A_ITALIC) ? ";3" : "");
        write_control(escape);
        while (cells < len &&
               (src[cells] & (A_ATTRIBUTES ^ A_ALTCHARSET)) == attr) {
            chtype ch = src[cells++];
            if ((ch & A_ALTCHARSET) && !(ch & 0xff80))
                ch = acs_map[ch & 0x7f];
            unsigned int code = ch & A_CHARTEXT;
            if (code == CMUS_WIDE_CONTINUATION) continue;
            WCHAR scalar[2];
            int count;
            const WCHAR *wide = win_curses_cluster(code, &count);
            if (!wide) {
                if (code > 0xffff && code <= 0x10ffff) {
                    scalar[0] = 0xd800 + ((code - 0x10000) >> 10);
                    scalar[1] = 0xdc00 + ((code - 0x10000) & 0x3ff);
                    count = 2;
                } else {
                    scalar[0] = code <= 0xffff ? code : 0xfffd;
                    count = 1;
                }
                wide = scalar;
            }
            if (n + count > 512) {
                write_wide(text, n);
                n = 0;
            }
            if (count > 512) write_wide(wide, count);
            else {
                memcpy(text + n, wide, count * sizeof(*text));
                n += count;
            }
        }
        write_wide(text, n);
        src += cells;
        x += cells;
        len -= cells;
    }
}

void PDC_blink_text(void)
{
    /* wincon defaults to bright backgrounds, with actual blinking disabled. */
    pdc_last_blink = GetTickCount();
}
