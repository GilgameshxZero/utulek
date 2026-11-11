package main

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var localeKernel = syscall.NewLazyDLL("kernel32.dll")
var localeMap = localeKernel.NewProc("LCMapStringEx")
var localeDate = localeKernel.NewProc("GetDateFormatEx")
var localeTime = localeKernel.NewProc("GetTimeFormatEx")
var localeValid = localeKernel.NewProc("IsValidLocaleName")

type nativeLocale struct {
	collation, calendar *uint16
	byteOrder, cDate    bool
}

func windowsLocaleName(s string) (*uint16, error) {
	if s == "" {
		return nil, nil
	}
	if isCLocale(s) {
		return syscall.UTF16PtrFromString("")
	}
	s = strings.SplitN(strings.SplitN(s, ".", 2)[0], "@", 2)[0]
	suffix := ""
	for _, candidate := range []string{"_stroke", "_pronun", "_radstr", "_phoneb", "_technl", "_modern", "_tradnl"} {
		if strings.HasSuffix(s, candidate) {
			suffix = candidate
			s = strings.TrimSuffix(s, candidate)
			break
		}
	}
	s = strings.ReplaceAll(s, "_", "-") + suffix
	p, e := syscall.UTF16PtrFromString(s)
	if e != nil {
		return nil, e
	}
	valid, _, _ := localeValid.Call(uintptr(unsafe.Pointer(p)))
	if valid == 0 {
		return nil, fmt.Errorf("locale %q is unavailable on Windows", s)
	}
	return p, nil
}
func newNativeLocale(collation, calendar string) (*nativeLocale, error) {
	a, e := windowsLocaleName(collation)
	if e != nil {
		return nil, e
	}
	b, e := windowsLocaleName(calendar)
	if e != nil {
		return nil, e
	}
	return &nativeLocale{a, b, isCLocale(collation), isCLocale(calendar)}, nil
}
func (l *nativeLocale) key(s string) string {
	if l.byteOrder {
		return s
	}
	input, e := syscall.UTF16PtrFromString(s)
	if e != nil {
		return s
	}
	n, _, _ := localeMap.Call(uintptr(unsafe.Pointer(l.collation)), 0x400, uintptr(unsafe.Pointer(input)), ^uintptr(0), 0, 0, 0, 0, 0)
	if n == 0 {
		return s
	}
	out := make([]byte, int(n))
	ok, _, _ := localeMap.Call(uintptr(unsafe.Pointer(l.collation)), 0x400, uintptr(unsafe.Pointer(input)), ^uintptr(0), uintptr(unsafe.Pointer(&out[0])), n, 0, 0, 0)
	if ok == 0 {
		return s
	}
	return string(out)
}
func (l *nativeLocale) date(t time.Time, withTime bool, zoned bool) string {
	if l.cDate {
		if withTime {
			return t.Format("Mon Jan _2 15:04:05 2006")
		}
		return t.Format("01/02/06")
	}
	st := syscall.Systemtime{Year: uint16(t.Year()), Month: uint16(t.Month()), Day: uint16(t.Day()), DayOfWeek: uint16(t.Weekday()), Hour: uint16(t.Hour()), Minute: uint16(t.Minute()), Second: uint16(t.Second())}
	var out [512]uint16
	n, _, _ := localeDate.Call(uintptr(unsafe.Pointer(l.calendar)), 1, uintptr(unsafe.Pointer(&st)), 0, uintptr(unsafe.Pointer(&out[0])), 512, 0)
	if n == 0 {
		return t.Format("2006-01-02")
	}
	value := syscall.UTF16ToString(out[:])
	if withTime {
		n, _, _ = localeTime.Call(uintptr(unsafe.Pointer(l.calendar)), 0, uintptr(unsafe.Pointer(&st)), 0, uintptr(unsafe.Pointer(&out[0])), 512)
		if n != 0 {
			value += " " + syscall.UTF16ToString(out[:])
		}
	}
	return value
}
