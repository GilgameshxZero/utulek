package main

import "os"

func replaceFile(from, to string) error    { return os.Rename(from, to) }
func installNewFile(from, to string) error { return os.Link(from, to) }

func openDefaultEditor(path string) error { return os.ErrInvalid }
