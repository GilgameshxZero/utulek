package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const browserProfilePrefix = "browser-"

// A locked home can have only one bridge or login process. Any profile left by a
// killed process is therefore stale and safe to remove before starting a new one.
func cleanupScratch(home string) (int, error) {
	entries, e := os.ReadDir(home)
	if e != nil {
		if errors.Is(e, os.ErrNotExist) {
			return 0, nil
		}
		return 0, e
	}
	removed := 0
	var firstErr error
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), browserProfilePrefix) {
			continue
		}
		if e := removeScratch(filepath.Join(home, entry.Name())); e != nil {
			if firstErr == nil {
				firstErr = e
			}
			continue
		}
		removed++
	}
	return removed, firstErr
}

// Chromium children can still be winding down when the parent exits. Retry so a
// clean shutdown does not leave a profile behind for the next startup to clean.
func removeScratch(path string) error {
	var e error
	for attempt := 0; attempt < 20; attempt++ {
		if e = os.RemoveAll(path); e == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return e
}
