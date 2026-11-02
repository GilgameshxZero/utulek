package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"
	"unsafe"
)

const (
	keyUp = 0x110001 + iota
	keyDown
	keyLeft
	keyRight
	keyHome
	keyEnd
	keyDelete
	keyBackspace
	keyPageUp
	keyPageDown
	keyResize
	keyEnter
	keyF2
	keyF4
	keyF5
	keyF6
)
const (
	keyPasteStart = 0x220001
	keyPasteEnd   = 0x220002
	keyShiftEnter = 0x220003
	attrBold      = 1
	attrDim       = 2
	attrReverse   = 4
)

var kernel = syscall.NewLazyDLL("kernel32.dll")
var getMode = kernel.NewProc("GetConsoleMode")
var setMode = kernel.NewProc("SetConsoleMode")
var info = kernel.NewProc("GetConsoleScreenBufferInfo")
var inputMode, outputMode uint32
var terminalBytes chan byte
var terminalStop, terminalDone chan struct{}
var terminalThemeName string
var terminalScreen strings.Builder
var lastFrame string
var pending []byte

func terminalInit() error {
	lastFrame = ""
	r, _, _ := getMode.Call(uintptr(os.Stdin.Fd()), uintptr(unsafe.Pointer(&inputMode)))
	if r == 0 {
		return errors.New("open smsgatec in a Windows terminal")
	}
	r, _, _ = getMode.Call(uintptr(os.Stdout.Fd()), uintptr(unsafe.Pointer(&outputMode)))
	if r == 0 {
		return errors.New("terminal output unavailable")
	}
	// VT input enables bracketed paste and mapped modified keys without enabling
	// enhanced keyboard protocols that intercept VS Code shortcuts.
	r, _, _ = setMode.Call(uintptr(os.Stdin.Fd()), uintptr((inputMode&^(1|2|4|0x40))|0x200|0x80))
	if r == 0 {
		return errors.New("cannot enable terminal input")
	}
	r, _, _ = setMode.Call(uintptr(os.Stdout.Fd()), uintptr(outputMode|4))
	if r == 0 {
		setMode.Call(uintptr(os.Stdin.Fd()), uintptr(inputMode))
		return errors.New("cannot enable VT output")
	}
	if terminalBytes == nil {
		terminalBytes = make(chan byte, 8192)
		terminalStop = make(chan struct{})
		terminalDone = make(chan struct{})
		bytes, stop, done := terminalBytes, terminalStop, terminalDone
		go func() {
			defer close(done)
			defer close(bytes)
			var buf [4096]byte
			for {
				select {
				case <-stop:
					return
				default:
				}
				n, e := os.Stdin.Read(buf[:])
				for _, b := range buf[:n] {
					select {
					case bytes <- b:
					case <-stop:
						return
					}
				}
				if e != nil {
					return
				}
			}
		}()
	}
	fmt.Print("\x1b[?1049h\x1b[?2004h\x1b[?25l")
	return nil
}
func terminalEnd() {
	if terminalStop != nil {
		close(terminalStop)
		for {
			kernel.NewProc("CancelIoEx").Call(uintptr(os.Stdin.Fd()), 0)
			select {
			case <-terminalDone:
				terminalBytes = nil
				terminalStop = nil
				pending = nil
				goto stopped
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
stopped:

	fmt.Print("\x1b[0m\x1b[?2004l\x1b[?25h\x1b[?1049l")
	setMode.Call(uintptr(os.Stdin.Fd()), uintptr(inputMode))
	setMode.Call(uintptr(os.Stdout.Fd()), uintptr(outputMode))
}
func terminalSuspend() { terminalEnd() }
func terminalResume()  { _ = terminalInit() }
func terminalSize() (int, int) {
	var b struct {
		Size, Cursor struct{ X, Y int16 }
		Attributes   uint16
		Window       struct{ Left, Top, Right, Bottom int16 }
		Max          struct{ X, Y int16 }
	}
	r, _, _ := info.Call(uintptr(os.Stdout.Fd()), uintptr(unsafe.Pointer(&b)))
	if r == 0 {
		return 24, 80
	}
	return int(b.Window.Bottom - b.Window.Top + 1), int(b.Window.Right - b.Window.Left + 1)
}
func terminalTheme(s string) { terminalThemeName = s }
func terminalClear() {
	terminalScreen.Reset()
	terminalScreen.WriteString("\x1b[?25l\x1b[0m\x1b[2J\x1b[H")
}
func terminalFlush() {
	frame := terminalScreen.String()
	if frame != lastFrame {
		fmt.Print(frame)
		lastFrame = frame
	}
}
func terminalCursor(y, x int) { fmt.Fprintf(&terminalScreen, "\x1b[%d;%dH\x1b[?25h", y+1, x+1) }
func put(y, x int, s string, attr int) {
	h, w := terminalSize()
	if y < 0 || y >= h || x < 0 || x >= w-1 {
		return
	}
	style := "\x1b[0m"
	if terminalThemeName == "light" {
		style += "\x1b[30;47m"
	}
	if terminalThemeName == "dark" {
		style += "\x1b[37;40m"
	}
	if attr&attrBold != 0 {
		style += "\x1b[1m"
	}
	if attr&attrDim != 0 {
		style += "\x1b[2m"
	}
	if attr&attrReverse != 0 {
		style += "\x1b[7m"
	}
	fmt.Fprintf(&terminalScreen, "\x1b[%d;%dH%s%s", y+1, x+1, style, clip(s, w-x-1))
}
func runeWidth(r rune) int {
	if r == 0 || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) {
		return 0
	}
	if r >= 0x300 && r <= 0x36f || r >= 0xfe00 && r <= 0xfe0f || r == 0x200d {
		return 0
	}
	if r >= 0x1100 && (r <= 0x115f || r >= 0x2e80 && r <= 0xa4cf || r >= 0xac00 && r <= 0xd7a3 || r >= 0xf900 && r <= 0xfaff || r >= 0xff01 && r <= 0xff60 || r >= 0x1f300) {
		return 2
	}
	return 1
}
func nextByte(wait time.Duration) (byte, bool) {
	if len(pending) > 0 {
		b := pending[0]
		pending = pending[1:]
		return b, true
	}
	select {
	case b, ok := <-terminalBytes:
		return b, ok
	case <-time.After(wait):
		return 0, false
	}
}
func terminalRead() int {
	b, ok := nextByte(80 * time.Millisecond)
	if !ok {
		return -1
	}
	if b == 27 {
		c, ok := nextByte(15 * time.Millisecond)
		if !ok {
			return 27
		}
		if c != '[' && c != 'O' {
			pending = append([]byte{c}, pending...)
			return 27
		}
		seq := []byte{c}
		for len(seq) < 64 {
			c, ok = nextByte(15 * time.Millisecond)
			if !ok {
				break
			}
			seq = append(seq, c)
			if c >= 64 && c <= 126 {
				break
			}
		}
		keys := map[string]int{"[A": keyUp, "OA": keyUp, "[B": keyDown, "OB": keyDown, "[C": keyRight, "OC": keyRight, "[D": keyLeft, "OD": keyLeft, "[H": keyHome, "OH": keyHome, "[F": keyEnd, "OF": keyEnd, "[1~": keyHome, "[4~": keyEnd, "[3~": keyDelete, "[5~": keyPageUp, "[6~": keyPageDown, "OQ": keyF2, "OS": keyF4, "[12~": keyF2, "[14~": keyF4, "[15~": keyF5, "[17~": keyF6, "[200~": keyPasteStart, "[201~": keyPasteEnd, "[13;2u": keyShiftEnter, "[27;2;13~": keyShiftEnter, "[13u": 13, "[27u": 27}
		if k, ok := keys[string(seq)]; ok {
			return k
		}
		return -1
	}
	if b < 128 {
		return int(b)
	}
	seq := []byte{b}
	for !utf8.FullRune(seq) && len(seq) < 4 {
		c, ok := nextByte(15 * time.Millisecond)
		if !ok {
			break
		}
		seq = append(seq, c)
	}
	r, _ := utf8.DecodeRune(seq)
	return int(r)
}

// Windows ships SQLite as winsqlite3.dll. Use its native API so the Windows
// build needs no C compiler or additional DLL and retains the Linux cache format.
var sqlDLL = syscall.NewLazyDLL("winsqlite3.dll")
var sqlOpen = sqlDLL.NewProc("sqlite3_open")
var sqlClose = sqlDLL.NewProc("sqlite3_close")
var sqlExec = sqlDLL.NewProc("sqlite3_exec")
var sqlPrepare = sqlDLL.NewProc("sqlite3_prepare_v2")
var sqlStep = sqlDLL.NewProc("sqlite3_step")
var sqlColumn = sqlDLL.NewProc("sqlite3_column_text")
var sqlFinalize = sqlDLL.NewProc("sqlite3_finalize")

func cstr(s string) []byte { return append([]byte(s), 0) }
func ptr(b []byte) uintptr { return uintptr(unsafe.Pointer(&b[0])) }
func fromC(p uintptr) string {
	if p == 0 {
		return ""
	}
	var b []byte
	for i := uintptr(0); ; i++ {
		c := *(*byte)(unsafe.Pointer(p + i))
		if c == 0 {
			break
		}
		b = append(b, c)
	}
	return string(b)
}

type Cache struct {
	revision uint64
	db       uintptr
	records  map[string]Message
	path     string
}

func (c *Cache) exec(s string) error {
	b := cstr(s)
	r, _, _ := sqlExec.Call(c.db, ptr(b), 0, 0, 0)
	if r != 0 {
		return errors.New("cannot write message cache")
	}
	return nil
}
func openCache(config Config, demo bool) (*Cache, error) {
	path := cachePath(config)
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
	}
	if path != "" {
		f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if e != nil {
			return nil, e
		}
		f.Close()
		if e = protectFile(path); e != nil {
			return nil, e
		}
	}
	c := &Cache{records: map[string]Message{}, path: path}
	b := cstr(actual)
	r, _, e := sqlOpen.Call(ptr(b), uintptr(unsafe.Pointer(&c.db)))
	if r != 0 {
		c.close()
		return nil, fmt.Errorf("cannot open SQLite cache: %v", e)
	}
	if err := c.exec("CREATE TABLE IF NOT EXISTS messages (key TEXT PRIMARY KEY, payload TEXT NOT NULL)"); err != nil {
		c.close()
		return nil, err
	}
	var stmt uintptr
	q := cstr("SELECT payload FROM messages")
	r, _, _ = sqlPrepare.Call(c.db, ptr(q), ^uintptr(0), uintptr(unsafe.Pointer(&stmt)), 0)
	if r != 0 {
		c.close()
		return nil, errors.New("cannot read cache")
	}
	defer sqlFinalize.Call(stmt)
	for {
		r, _, _ = sqlStep.Call(stmt)
		if r == 101 {
			break
		}
		if r != 100 {
			c.close()
			return nil, errors.New("cannot read cached messages")
		}
		p, _, _ := sqlColumn.Call(stmt, 0)
		var m Message
		if err := json.Unmarshal([]byte(fromC(p)), &m); err != nil {
			c.close()
			return nil, err
		}
		c.records[m.key()] = m
	}
	return c, nil
}
func (c *Cache) put(messages []Message) error {
	messages = changedMessages(c.records, messages)
	if len(messages) == 0 {
		return nil
	}
	if err := c.exec("BEGIN"); err != nil {
		return err
	}
	for _, m := range messages {
		b, err := json.Marshal(m)
		if err != nil {
			_ = c.exec("ROLLBACK")
			return err
		}
		quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
		if err = c.exec("INSERT OR REPLACE INTO messages VALUES (" + quote(m.key()) + "," + quote(string(b)) + ")"); err != nil {
			_ = c.exec("ROLLBACK")
			return err
		}
	}
	if err := c.exec("COMMIT"); err != nil {
		_ = c.exec("ROLLBACK")
		return err
	}
	for _, m := range messages {
		c.records[m.key()] = m
	}
	c.revision++
	return nil
}
func (c *Cache) close() {
	if c.db != 0 {
		sqlClose.Call(c.db)
		c.db = 0
	}
}
