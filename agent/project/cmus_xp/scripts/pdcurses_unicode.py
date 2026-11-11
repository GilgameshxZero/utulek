"""Generate a private Unicode-capable ABI for the pinned static PDCurses build.

Keep the dependency source untouched. Attributes still fit in the application's
int options; only color pairs need the full curses attr_t type.
"""
import re


def prepare(pdc, out):
    text = (pdc / 'curses.h').read_text()
    text, count = re.subn(r'#if _LP64\ntypedef unsigned int chtype;.*?\n#endif',
                         'typedef unsigned long long chtype;', text, count=1, flags=re.S)
    if count != 1:
        raise RuntimeError('Unexpected pinned PDCurses chtype declaration')
    for name in ['A_ALTCHARSET', 'A_RIGHT', 'A_LEFT', 'A_ITALIC', 'A_UNDERLINE',
                 'A_REVERSE', 'A_BLINK', 'A_BOLD', 'A_ATTRIBUTES', 'A_COLOR']:
        pattern = r'(#define ' + name + r'\s+\(chtype\))0x([0-9a-f]+)'
        text, count = re.subn(pattern, lambda m: m[1] + hex(int(m[2], 16) << 5) + 'ULL', text)
        if count != 1:
            raise RuntimeError('Unexpected PDCurses attribute: ' + name)
    text = text.replace('(chtype)0x0000ffff', '(chtype)0x001fffffULL')
    text = text.replace('#define PDC_COLOR_SHIFT 24', '#define PDC_COLOR_SHIFT 29')
    text = text.replace('#define A_STANDOUT    (A_REVERSE | A_BOLD)',
                        '#define A_STANDOUT    A_REVERSE')
    header = out / 'unicode_include/curses.h'
    header.parent.mkdir(exist_ok=True)
    header.write_text(text)
    text = (pdc / 'pdcurses/addstr.c').read_text()
    begin, end = text.index('int waddnstr(' , text.index('**man-end')), text.index('\nint addstr(')
    text = text[:begin] + '''#include "windows/curses_unicode.h"
int waddnstr(WINDOW *win, const char *str, int n)
{
    return win_curses_waddnstr(win, str, n);
}
''' + text[end:]
    source = out / 'pdc_addstr_unicode.c'
    source.write_text(text)
    return header, source
