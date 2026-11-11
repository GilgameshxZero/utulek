package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func localeName(category string) string {
	for _, key := range []string{"LC_ALL", category, "LANG"} {
		if s := os.Getenv(key); s != "" {
			return s
		}
	}
	return ""
}
func isCLocale(s string) bool {
	return s == "C" || s == "POSIX" || strings.EqualFold(s, "C.UTF-8") || strings.EqualFold(s, "C.utf8")
}
func (c Config) collationKey(s string) string {
	if c.Locale == nil {
		return strings.ToLower(s)
	}
	return c.Locale.key(strings.ToLower(s))
}
func formatContactDate(c *Card, field string, cfg Config) string {
	date, text, present, zoned := parsedSearchDate(c, field)
	if !present {
		return ""
	}
	if text != nil {
		return *text
	}
	if (date.Year() == 1900 || c.reducedDate(field)) && date.Hour() == 0 && date.Minute() == 0 && date.Second() == 0 && date.Nanosecond() == 0 && !zoned {
		return date.Format("--01-02")
	}
	_, offset := date.Zone()
	withTime := date.Hour() != 0 || date.Minute() != 0 || date.Second() != 0 || date.Nanosecond() != 0 || offset != 0
	if cfg.LocalizeDates && cfg.Locale != nil {
		return cfg.Locale.date(date, withTime, zoned)
	}
	if withTime {
		return date.Format("2006-01-02T15:04:05.999999999-07:00")
	}
	return date.Format("2006-01-02")
}
func isoModelDate(c *Card, field string) any {
	date, text, present, zoned := parsedSearchDate(c, field)
	if !present {
		return nil
	}
	if text != nil {
		return "text= " + *text
	}
	if c.first("VERSION") == "4.0" && (date.Year() == 1900 || c.reducedDate(field)) && date.Hour() == 0 && date.Minute() == 0 && date.Second() == 0 {
		return date.Format("--01-02")
	}
	if zoned {
		return date.Format(time.RFC3339Nano)
	}
	if date.Hour() != 0 || date.Minute() != 0 || date.Second() != 0 || date.Nanosecond() != 0 {
		return date.Format("2006-01-02T15:04:05.999999999")
	}
	return date.Format("2006-01-02")
}
func checkedLocale() (*nativeLocale, error) {
	loc, e := newNativeLocale(localeName("LC_COLLATE"), localeName("LC_TIME"))
	if e != nil {
		return nil, fmt.Errorf("locale: %w", e)
	}
	return loc, nil
}

func (c *Card) reducedDate(field string) bool {
	key := "BDAY"
	if field == "anniversary" {
		key = "ANNIVERSARY"
	}
	for _, p := range c.Props {
		if p.Key == key {
			return strings.HasPrefix(p.Value, "--")
		}
	}
	if field == "anniversary" {
		for _, p := range c.Props {
			if p.Key == "X-ANNIVERSARY" {
				return strings.HasPrefix(p.Value, "--")
			}
		}
	}
	return false
}
