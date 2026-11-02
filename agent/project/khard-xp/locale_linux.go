//go:build linux && cgo

package main

/*
#define _GNU_SOURCE
#include <locale.h>
#include <langinfo.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
static locale_t khard_locale(const char *name, int category) {
 return newlocale(category == 0 ? LC_COLLATE_MASK : LC_TIME_MASK, name[0] ? name : "C", (locale_t)0);
}
static size_t khard_date(char *out, size_t size, locale_t loc, int y,int m,int d,int h,int min,int sec,int weekday,int yday,int with_time,int offset,const char *zone,int zoned) {
 struct tm value = {0}; value.tm_year=y-1900;value.tm_mon=m-1;value.tm_mday=d;
 value.tm_hour=h;value.tm_min=min;value.tm_sec=sec;value.tm_wday=weekday;value.tm_yday=yday-1;value.tm_isdst=zoned ? 0 : -1;value.tm_gmtoff=offset;value.tm_zone=zone;
 return strftime_l(out,size,nl_langinfo_l(with_time ? D_T_FMT : D_FMT,loc),&value,loc);
}
*/
import "C"
import (
	"fmt"
	"runtime"
	"time"
	"unsafe"
)

type nativeLocale struct {
	collation, calendar C.locale_t
	byteOrder           bool
}

func newNativeLocale(collation, calendar string) (*nativeLocale, error) {
	out := &nativeLocale{byteOrder: collation == "" || isCLocale(collation)}
	for i, name := range []string{collation, calendar} {
		s := C.CString(name)
		handle := C.khard_locale(s, C.int(i))
		C.free(unsafe.Pointer(s))
		if handle == nil {
			if out.collation != nil {
				C.freelocale(out.collation)
			}
			return nil, fmt.Errorf("locale %q is not installed", name)
		}
		if i == 0 {
			out.collation = handle
		} else {
			out.calendar = handle
		}
	}
	runtime.SetFinalizer(out, func(l *nativeLocale) { C.freelocale(l.collation); C.freelocale(l.calendar) })
	return out, nil
}
func (l *nativeLocale) key(s string) string {
	if l.byteOrder {
		return s
	}
	input := C.CString(s)
	defer C.free(unsafe.Pointer(input))
	n := C.strxfrm_l(nil, input, 0, l.collation)
	buffer := make([]byte, int(n)+1)
	C.strxfrm_l((*C.char)(unsafe.Pointer(&buffer[0])), input, C.size_t(len(buffer)), l.collation)
	runtime.KeepAlive(l)
	return string(buffer[:int(n)])
}
func (l *nativeLocale) date(t time.Time, withTime bool, zoned bool) string {
	var out [512]byte
	flag := 0
	if withTime {
		flag = 1
	}
	zone := ""
	_, offset := t.Zone()
	flagZ := 0
	if zoned {
		flagZ = 1
		zone = "UTC"
		if offset != 0 {
			zone += "" + t.Format("-07:00")
		}
	}
	zoneC := C.CString(zone)
	defer C.free(unsafe.Pointer(zoneC))
	n := C.khard_date((*C.char)(unsafe.Pointer(&out[0])), 512, l.calendar, C.int(t.Year()), C.int(t.Month()), C.int(t.Day()), C.int(t.Hour()), C.int(t.Minute()), C.int(t.Second()), C.int(t.Weekday()), C.int(t.YearDay()), C.int(flag), C.int(offset), zoneC, C.int(flagZ))
	runtime.KeepAlive(l)
	if n == 0 {
		return t.Format("2006-01-02")
	}
	return string(out[:int(n)])
}
