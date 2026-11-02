package main

import (
	"testing"
	"time"
)

func TestWindowsVTInput(t *testing.T) {
	original := terminalBytes
	defer func() { terminalBytes = original; pending = nil }()
	cases := []struct {
		input string
		want  int
	}{{"\x1b[A", keyUp}, {"\x1bOB", keyDown}, {"\x1b[13;2u", keyShiftEnter}, {"\x1b[200~", keyPasteStart}, {"\x1b[201~", keyPasteEnd}, {"\x1b[14~", keyF4}, {"\x1b[99;6u", -1}, {"界", int('界')}, {"é", int('é')}, {"\r", 13}, {"\x7f", 127}, {"\x1b", 27}}
	for _, c := range cases {
		terminalBytes = make(chan byte, 100)
		pending = nil
		for _, b := range []byte(c.input) {
			terminalBytes <- b
		}
		if got := terminalRead(); got != c.want {
			t.Fatalf("%q: got %d want %d", c.input, got, c.want)
		}
	}
	terminalBytes = make(chan byte, 1)
	start := time.Now()
	if terminalRead() != -1 || time.Since(start) > time.Second {
		t.Fatal("idle input does not yield to refresh")
	}
}

func TestWindowsEditorPath(t *testing.T) {
	args, e := splitCommand(`"C:\Program Files\Editor\editor.exe" --wait`)
	if e != nil || len(args) != 2 || args[0] != `C:\Program Files\Editor\editor.exe` || args[1] != "--wait" {
		t.Fatal(args, e)
	}
}
