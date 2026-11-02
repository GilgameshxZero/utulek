package main

/*
#cgo pkg-config: ncursesw sqlite3
#define _XOPEN_SOURCE_EXTENDED 1
#include <ncursesw/curses.h>
#include <sqlite3.h>
#include <locale.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <wchar.h>

static int sg_init(void) {
    setlocale(LC_ALL, "");
    if (!initscr()) return -1;
    raw(); noecho(); keypad(stdscr, TRUE); timeout(80); set_escdelay(25);
    if (has_colors()) {start_color(); use_default_colors();}
    // Leave enhanced keyboard protocols disabled so the terminal/editor owns
    // shortcuts such as Ctrl+Shift+C. Still accept mapped Shift+Enter input.
    fputs("\033[?2004h", stdout); fflush(stdout);
    return 0;
}
static void sg_end(void) {
    fputs("\033[?2004l", stdout); fflush(stdout); endwin();
}
static int sg_height(void) {return getmaxy(stdscr);}
static int sg_width(void) {return getmaxx(stdscr);}
static void sg_put(int y,int x,const char *s,int attr) {attrset(attr|COLOR_PAIR(1)); mvaddstr(y,x,s);}
static void sg_clear(void) {erase();curs_set(0);}
static void sg_flush(void) {refresh();}
static void sg_cursor(int y,int x) {curs_set(1);move(y,x);}
static void sg_theme(int theme) {
    if (has_colors()) {
        short fg=-1,bg=-1;
        if(theme==1){fg=COLOR_BLACK;bg=COLOR_WHITE;}
        if(theme==2){fg=COLOR_WHITE;bg=COLOR_BLACK;}
        init_pair(1,fg,bg);bkgd(' '|COLOR_PAIR(1));
    }
}
static void sg_suspend(void) {def_prog_mode();sg_end();}
static void sg_resume(void) {reset_prog_mode();keypad(stdscr,TRUE);timeout(80);clearok(stdscr,TRUE);fputs("\033[?2004h",stdout);fflush(stdout);}
// Accept both normal (CSI) and application (SS3) cursor keys. A terminal
// may send either form regardless of the active terminfo keypad settings.
static int sg_cursor_key(const wchar_t *tail) {
    int n=wcslen(tail);
    if(n<2 || (tail[0]!='[' && tail[0]!='O'))return -1;
    // Modified arrows are not aliases for the application's plain arrows.
    if(n!=2 && (n!=5 || tail[1]!='1' || tail[2]!=';' || tail[3]!='1'))return -1;
    int k;
    switch(tail[n-1]) {
        case 'A': k=KEY_UP;break;
        case 'B': k=KEY_DOWN;break;
        case 'C': k=KEY_RIGHT;break;
        case 'D': k=KEY_LEFT;break;
        case 'H': k=KEY_HOME;break;
        case 'F': k=KEY_END;break;
        default:return -1;
    }
    return 0x110000+k;
}
// Special keys are outside the Unicode range. Terminal protocols distinguish
// shifted Enter and bracketed paste, so pasted newlines cannot activate send.
static int sg_read(void) {
    wint_t k; int status=get_wch(&k);
    if(status==ERR)return -1;
    if(status==KEY_CODE_YES)return 0x110000+(int)k;
    if(k!=27)return (int)k;
    wchar_t tail[64];int n=0;
    timeout(15);
    while(n<63) {
        wint_t c;int s=get_wch(&c);
        if(s==ERR)break;
        if(s==KEY_CODE_YES){ungetch(c);break;}
        tail[n++]=(wchar_t)c;
        if(n==1&&c!='['&&c!='O')break;
        if(n>1&&c>=64&&c<=126)break;
    }
    timeout(80);tail[n]=0;
    int cursor=sg_cursor_key(tail);
    if(cursor>=0)return cursor;
    if(!wcscmp(tail,L"[200~"))return 0x220001;
    if(!wcscmp(tail,L"[201~"))return 0x220002;
    if(!wcscmp(tail,L"[13;2u")||!wcscmp(tail,L"[27;2;13~"))return 0x220003;
    if(!wcscmp(tail,L"[13u")||!wcscmp(tail,L"[13;1u"))return 13;
    if(!wcscmp(tail,L"[27u"))return 27;
    // Unknown modified keys must not turn into Escape plus typed characters
    // (which could finish compose or activate an unrelated command).
    if(n>0 && (tail[0]=='[' || tail[0]=='O'))return -1;
    for(int i=n-1;i>=0;i--)unget_wch(tail[i]);
    return 27;
}
static int sg_wcwidth(int r) {return wcwidth((wchar_t)r);}
static int sg_bind(sqlite3_stmt *stmt,int i,const char *s) {return sqlite3_bind_text(stmt,i,s,-1,SQLITE_TRANSIENT);}
*/
import "C"

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"unsafe"
)

const (
	keyUp         = 0x110000 + int(C.KEY_UP)
	keyDown       = 0x110000 + int(C.KEY_DOWN)
	keyLeft       = 0x110000 + int(C.KEY_LEFT)
	keyRight      = 0x110000 + int(C.KEY_RIGHT)
	keyHome       = 0x110000 + int(C.KEY_HOME)
	keyEnd        = 0x110000 + int(C.KEY_END)
	keyDelete     = 0x110000 + int(C.KEY_DC)
	keyBackspace  = 0x110000 + int(C.KEY_BACKSPACE)
	keyPageUp     = 0x110000 + int(C.KEY_PPAGE)
	keyPageDown   = 0x110000 + int(C.KEY_NPAGE)
	keyResize     = 0x110000 + int(C.KEY_RESIZE)
	keyEnter      = 0x110000 + int(C.KEY_ENTER)
	keyF2         = 0x110000 + int(C.KEY_F0) + 2
	keyF4         = 0x110000 + int(C.KEY_F0) + 4
	keyF5         = 0x110000 + int(C.KEY_F0) + 5
	keyF6         = 0x110000 + int(C.KEY_F0) + 6
	keyPasteStart = 0x220001
	keyPasteEnd   = 0x220002
	keyShiftEnter = 0x220003
	attrBold      = int(C.A_BOLD)
	attrDim       = int(C.A_DIM)
	attrReverse   = int(C.A_REVERSE)
)

func terminalInit() error {
	if C.sg_init() != 0 {
		return errors.New("cannot initialize terminal")
	}
	return nil
}
func terminalEnd()             { C.sg_end() }
func terminalSize() (int, int) { return int(C.sg_height()), int(C.sg_width()) }
func terminalRead() int        { return int(C.sg_read()) }
func terminalClear()           { C.sg_clear() }
func terminalFlush()           { C.sg_flush() }
func terminalCursor(y, x int)  { C.sg_cursor(C.int(y), C.int(x)) }
func terminalSuspend()         { C.sg_suspend() }
func terminalResume()          { C.sg_resume() }
func terminalTheme(theme string) {
	n := 0
	if theme == "light" {
		n = 1
	}
	if theme == "dark" {
		n = 2
	}
	C.sg_theme(C.int(n))
}
func runeWidth(r rune) int {
	n := int(C.sg_wcwidth(C.int(r)))
	if n < 0 {
		return 1
	}
	return n
}
func put(y, x int, s string, attr int) {
	h, w := terminalSize()
	if y < 0 || y >= h || x < 0 || x >= w-1 {
		return
	}
	text := C.CString(clip(s, w-x-1))
	defer C.free(unsafe.Pointer(text))
	C.sg_put(C.int(y), C.int(x), text, C.int(attr))
}

type Cache struct {
	revision uint64
	db       *C.sqlite3
	records  map[string]Message
	path     string
}

func openCache(c Config, demo bool) (*Cache, error) {
	path := cachePath(c)
	if demo {
		path = ""
	}
	actual := path
	if actual == "" {
		actual = ":memory:"
	} else {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return nil, err
		}
		f.Close()
		if err = os.Chmod(path, 0600); err != nil {
			return nil, err
		}
	}
	name := C.CString(actual)
	defer C.free(unsafe.Pointer(name))
	cache := &Cache{records: map[string]Message{}, path: path}
	if C.sqlite3_open(name, &cache.db) != C.SQLITE_OK {
		if cache.db != nil {
			C.sqlite3_close(cache.db)
		}
		return nil, errors.New("cannot open message cache")
	}
	C.sqlite3_busy_timeout(cache.db, 2000)
	if err := cache.exec("CREATE TABLE IF NOT EXISTS messages (key TEXT PRIMARY KEY, payload TEXT NOT NULL)"); err != nil {
		cache.close()
		return nil, err
	}
	var stmt *C.sqlite3_stmt
	query := C.CString("SELECT payload FROM messages")
	defer C.free(unsafe.Pointer(query))
	if C.sqlite3_prepare_v2(cache.db, query, -1, &stmt, nil) != C.SQLITE_OK {
		cache.close()
		return nil, errors.New("cannot read message cache")
	}
	defer C.sqlite3_finalize(stmt)
	for {
		status := C.sqlite3_step(stmt)
		if status == C.SQLITE_DONE {
			break
		}
		if status != C.SQLITE_ROW {
			cache.close()
			return nil, errors.New("cannot read cached messages")
		}
		data := C.GoString((*C.char)(unsafe.Pointer(C.sqlite3_column_text(stmt, 0))))
		var message Message
		if json.Unmarshal([]byte(data), &message) != nil {
			return nil, errors.New("invalid cached message")
		}
		cache.records[message.key()] = message
	}
	return cache, nil
}
func (c *Cache) exec(sql string) error {
	query := C.CString(sql)
	defer C.free(unsafe.Pointer(query))
	if C.sqlite3_exec(c.db, query, nil, nil, nil) != C.SQLITE_OK {
		return errors.New("message cache write failed")
	}
	return nil
}
func (c *Cache) put(messages []Message) error {
	messages = changedMessages(c.records, messages)
	if len(messages) == 0 {
		return nil
	}
	if err := c.exec("BEGIN"); err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			_ = c.exec("ROLLBACK")
		}
	}()
	var stmt *C.sqlite3_stmt
	query := C.CString("INSERT OR REPLACE INTO messages VALUES (?,?)")
	defer C.free(unsafe.Pointer(query))
	if C.sqlite3_prepare_v2(c.db, query, -1, &stmt, nil) != C.SQLITE_OK {
		return errors.New("cannot save message")
	}
	defer C.sqlite3_finalize(stmt)
	for _, message := range messages {
		data, _ := json.Marshal(message)
		key := C.CString(message.key())
		payload := C.CString(string(data))
		C.sqlite3_reset(stmt)
		C.sqlite3_clear_bindings(stmt)
		a := C.sg_bind(stmt, 1, key)
		b := C.sg_bind(stmt, 2, payload)
		C.free(unsafe.Pointer(key))
		C.free(unsafe.Pointer(payload))
		if a != C.SQLITE_OK || b != C.SQLITE_OK || C.sqlite3_step(stmt) != C.SQLITE_DONE {
			return errors.New("cannot save outgoing message")
		}
	}
	if err := c.exec("COMMIT"); err != nil {
		return err
	}
	ok = true
	for _, m := range messages {
		c.records[m.key()] = m
	}
	c.revision++
	return nil
}
func (c *Cache) close() {
	if c != nil && c.db != nil {
		C.sqlite3_close(c.db)
		c.db = nil
	}
}
