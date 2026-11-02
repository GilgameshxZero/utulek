package main

import "os"

func protectFile(path string) error { return os.Chmod(path, 0600) }
func isPrivateFile(path string) bool {
	s, e := os.Stat(path)
	return e == nil && s.Mode().Perm() == 0600
}
