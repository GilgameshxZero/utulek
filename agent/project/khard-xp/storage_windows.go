package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var moveFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func moveFile(from, to string, flags uintptr) error {
	f, e := syscall.UTF16PtrFromString(from)
	if e != nil {
		return e
	}
	t, e := syscall.UTF16PtrFromString(to)
	if e != nil {
		return e
	}
	r, _, e := moveFileEx.Call(uintptr(unsafe.Pointer(f)), uintptr(unsafe.Pointer(t)), flags)
	if r == 0 {
		return e
	}
	return nil
}
func replaceFile(from, to string) error    { return moveFile(from, to, 0x1|0x8) } // REPLACE_EXISTING | WRITE_THROUGH
func installNewFile(from, to string) error { return moveFile(from, to, 0x8) }

// ShellExecute uses the user's file association without a command shell.
func openDefaultEditor(path string) error {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	verb, _ := syscall.UTF16PtrFromString("open")
	proc := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")
	result, _, _ := proc.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(p)), 0, 0, 1)
	if result <= 32 {
		return fmt.Errorf("opening text file failed (ShellExecute code %d)", result)
	}
	return nil
}
