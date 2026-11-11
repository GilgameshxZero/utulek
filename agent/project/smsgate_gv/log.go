package main

import (
	"os"
)

// Shell redirection opens the log with O_APPEND, so truncating at startup is
// safe and keeps the open descriptor usable without a rename dance.
func rotateLogFile(file *os.File, limit int64) error {
	if file == nil {
		return nil
	}
	info, e := file.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Size() <= limit {
		return e
	}
	if e = file.Truncate(0); e != nil {
		return e
	}
	_, e = file.Seek(0, 0)
	return e
}
