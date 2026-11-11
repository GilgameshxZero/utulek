package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestTransferRetainsBothOnRemovalFailure(t *testing.T) {
	_, _, source, book := operationBooks(t)
	// Permit reads and writes but deny deletion, as an external Windows app can.
	path, e := syscall.UTF16PtrFromString(source.Path)
	if e != nil {
		t.Fatal(e)
	}
	handle, e := syscall.CreateFile(path, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, 0, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer syscall.CloseHandle(handle)
	e = transferCard(source, book, true)
	if e == nil || !strings.Contains(e.Error(), "source retained") {
		t.Fatal("missing partial completion diagnostic", e)
	}
	if e = sameOriginal(source); e != nil {
		t.Fatal("source changed", e)
	}
	data, e := os.ReadFile(filepath.Join(book.Path, source.first("UID")+".vcf"))
	if e != nil {
		t.Fatal("destination lost", e)
	}
	if !bytes.Contains(data, []byte("UID:example-one")) || !bytes.Contains(data, []byte("X-EXTRA;X-PARAM=preserve:")) {
		t.Fatal("destination corrupted")
	}
}
