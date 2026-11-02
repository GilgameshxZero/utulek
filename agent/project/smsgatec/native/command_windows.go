package main

import (
	"errors"
	"strings"
	"syscall"
	"unsafe"
)

func splitCommand(input string) ([]string, error) {
	if strings.TrimSpace(input) == "" {
		return nil, errors.New("EDITOR is empty")
	}
	p, e := syscall.UTF16PtrFromString(input)
	if e != nil {
		return nil, e
	}
	var n int32
	argv, e := syscall.CommandLineToArgv(p, &n)
	if e != nil {
		return nil, e
	}
	defer syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(argv))))
	args := make([]string, int(n))
	for i := range args {
		args[i] = syscall.UTF16ToString(argv[i][:])
	}
	if len(args) == 0 || args[0] == "" {
		return nil, errors.New("EDITOR is empty")
	}
	return args, nil
}
