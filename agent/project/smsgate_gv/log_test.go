package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotateLogFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serve.log")
	file, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if e != nil {
		t.Fatal(e)
	}
	defer file.Close()
	if _, e = file.Write(make([]byte, 2048)); e != nil {
		t.Fatal(e)
	}
	if e = rotateLogFile(file, 1024); e != nil {
		t.Fatal(e)
	}
	info, e := file.Stat()
	if e != nil || info.Size() != 0 {
		t.Fatalf("size=%d err=%v", info.Size(), e)
	}
	if _, e = file.Write([]byte("after rotation\n")); e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(path)
	if e != nil || string(data) != "after rotation\n" {
		t.Fatalf("data=%q err=%v", data, e)
	}
}
