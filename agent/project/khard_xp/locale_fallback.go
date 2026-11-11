//go:build !windows && (!linux || !cgo)

package main

import (
	"fmt"
	"time"
)

type nativeLocale struct{}

func newNativeLocale(collation, calendar string) (*nativeLocale, error) {
	if (collation != "" && !isCLocale(collation)) || (calendar != "" && !isCLocale(calendar)) {
		return nil, fmt.Errorf("native locale support requires a Linux cgo build or Windows")
	}
	return &nativeLocale{}, nil
}
func (l *nativeLocale) key(s string) string { return s }
func (l *nativeLocale) date(t time.Time, withTime bool, zoned bool) string {
	if withTime {
		return t.Format("Mon Jan _2 15:04:05 2006")
	}
	return t.Format("01/02/06")
}
