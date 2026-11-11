package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func discoverBookPaths(pattern string) ([]string, error) {
	volume := filepath.VolumeName(pattern)
	rest := strings.TrimPrefix(pattern, volume)
	base := volume
	if filepath.IsAbs(pattern) {
		base += string(filepath.Separator)
		rest = strings.TrimLeft(rest, string(filepath.Separator))
	}
	parts := strings.Split(rest, string(filepath.Separator))
	hits := map[string]bool{}
	var walk func(string, int, int) error
	walk = func(dir string, index, depth int) error {
		if depth > 128 {
			return fmt.Errorf("address book discovery exceeds 128 directory levels")
		}
		if index == len(parts) {
			if info, e := os.Stat(dir); e == nil && info.IsDir() {
				hits[filepath.Clean(dir)] = true
			}
			return nil
		}
		part := parts[index]
		if part == "**" {
			if e := walk(dir, index+1, depth); e != nil {
				return e
			}
			entries, e := os.ReadDir(dir)
			if os.IsNotExist(e) {
				return nil
			}
			if e != nil {
				return e
			}
			for _, ent := range entries {
				if ent.IsDir() && !strings.HasPrefix(ent.Name(), ".") {
					if e := walk(filepath.Join(dir, ent.Name()), index, depth+1); e != nil {
						return e
					}
				}
			}
			return nil
		}
		matches, e := filepath.Glob(filepath.Join(dir, part))
		if e != nil {
			return e
		}
		for _, path := range matches {
			if strings.HasPrefix(filepath.Base(path), ".") && !strings.HasPrefix(part, ".") {
				continue
			}
			if e := walk(path, index+1, depth+1); e != nil {
				return e
			}
		}
		return nil
	}
	if e := walk(base, 0, 0); e != nil {
		return nil, e
	}
	parents := map[string]bool{}
	for dir := range hits {
		parents[filepath.Dir(dir)] = true
	}
	var paths []string
	for path := range hits {
		if !parents[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths, nil
}
func unfoldBooks(books []Book, types map[int]string) ([]Book, error) {
	reserved := map[string]bool{}
	var out []Book
	for i, b := range books {
		reserved[b.Name] = true
		if types[i] != "discover" {
			out = append(out, b)
		}
	}
	for i, b := range books {
		if types[i] != "discover" {
			continue
		}
		paths, e := discoverBookPaths(b.Path)
		if e != nil {
			return nil, fmt.Errorf("discover %s: %w", b.Name, e)
		}
		for _, path := range paths {
			name := filepath.Base(path)
			base := name
			for n := 1; reserved[name]; n++ {
				name = fmt.Sprintf("%s-%d", base, n)
			}
			reserved[name] = true
			out = append(out, Book{name, path})
		}
		delete(reserved, b.Name)
	}
	return out, nil
}
