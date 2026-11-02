package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func uuid() (string, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func sameOriginal(c *Card) error {
	info, e := os.Lstat(c.Path)
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to modify a symlink or non-regular contact file")
	}
	b, e := os.ReadFile(c.Path)
	if e != nil {
		return e
	}
	if !bytes.Equal(b, c.Original) {
		return fmt.Errorf("contact changed on disk; reload before trying again")
	}
	return nil
}

func withCardLock(c *Card, fn func() error) error {
	// Retain the original lock namespace for interoperability across upgrades.
	path := c.Path + ".khard-win.lock"
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return fmt.Errorf("cannot lock contact (another khard-xp may be editing it): %w", e)
	}
	f.Close()
	defer os.Remove(path)
	if e = sameOriginal(c); e != nil {
		return e
	}
	return fn()
}

func stageCard(c *Card, dir string) (path string, err error) {
	data := c.bytes()
	if _, err = parseCard(data); err != nil {
		return "", fmt.Errorf("refusing invalid vCard: %w", err)
	}
	f, err := os.CreateTemp(dir, ".khard-xp-*.tmp")
	if err != nil {
		return "", err
	}
	path = f.Name()
	defer func() {
		if err != nil {
			os.Remove(path)
		}
	}()
	if _, err = f.Write(data); err != nil {
		f.Close()
		return path, err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return path, err
	}
	err = f.Close()
	return path, err
}

func saveCard(c *Card) error {
	return withCardLock(c, func() error {
		c.replace("REV", []Property{newProp("REV", time.Now().UTC().Format("20060102T150405Z"))})
		path, e := stageCard(c, filepath.Dir(c.Path))
		if e != nil {
			return e
		}
		defer os.Remove(path)
		if info, e := os.Stat(c.Path); e == nil {
			if e = os.Chmod(path, info.Mode().Perm()); e != nil {
				return e
			}
		}
		if e = sameOriginal(c); e != nil {
			return e
		}
		if e = replaceFile(path, c.Path); e != nil {
			return e
		}
		c.Original = c.bytes()
		return nil
	})
}

func createCard(c *Card, b Book) error { return installCard(c, b, false, nil) }

// Share destination validation, staging and no-clobber installation for new,
// copied and moved contacts. The caller may recheck its source before install.
func installCard(c *Card, b Book, preserveUID bool, before func() error) error {
	info, e := os.Stat(b.Path)
	if e != nil {
		return fmt.Errorf("address book directory must already exist: %w", e)
	}
	if !info.IsDir() {
		return fmt.Errorf("address book path is not a directory")
	}
	id := c.first("UID")
	if !preserveUID || id == "" {
		id, e = uuid()
		if e != nil {
			return e
		}
		c.replace("UID", []Property{newProp("UID", id)})
	}
	if id == "." || id == ".." || strings.ContainsAny(id, "/\\:\x00") {
		return fmt.Errorf("UID cannot be used as a destination filename")
	}
	c.replace("REV", []Property{newProp("REV", time.Now().UTC().Format("20060102T150405Z"))})
	path, e := stageCard(c, b.Path)
	if e != nil {
		return e
	}
	defer os.Remove(path)
	dest := filepath.Join(b.Path, id+".vcf")
	if before != nil {
		if e = before(); e != nil {
			return e
		}
	}
	if e = installNewFile(path, dest); e != nil {
		return e
	}
	c.Path = dest
	c.Book = b.Name
	c.Original = c.bytes()
	return nil
}

func removeCard(c *Card) error { return withCardLock(c, func() error { return os.Remove(c.Path) }) }
