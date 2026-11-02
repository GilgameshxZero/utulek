package main

import (
	"strings"
	"syscall"
	"unsafe"
)

var security = syscall.NewLazyDLL("advapi32.dll")
var convertSD = security.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
var setSecurity = security.NewProc("SetFileSecurityW")
var getSecurity = security.NewProc("GetFileSecurityW")
var stringifySD = security.NewProc("ConvertSecurityDescriptorToStringSecurityDescriptorW")

func userSID() (string, error) {
	t, e := syscall.OpenCurrentProcessToken()
	if e != nil {
		return "", e
	}
	defer t.Close()
	u, e := t.GetTokenUser()
	if e != nil {
		return "", e
	}
	return u.User.Sid.String()
}
func protectFile(path string) error {
	sid, e := userSID()
	if e != nil {
		return e
	}
	s, e := syscall.UTF16PtrFromString("D:P(A;;FA;;;" + sid + ")")
	if e != nil {
		return e
	}
	var sd uintptr
	r, _, e := convertSD.Call(uintptr(unsafe.Pointer(s)), 1, uintptr(unsafe.Pointer(&sd)), 0)
	if r == 0 {
		return e
	}
	defer kernel.NewProc("LocalFree").Call(sd)
	p, e := syscall.UTF16PtrFromString(path)
	if e != nil {
		return e
	}
	r, _, e = setSecurity.Call(uintptr(unsafe.Pointer(p)), 0x80000004, sd)
	if r == 0 {
		return e
	}
	return nil
}
func isPrivateFile(path string) bool {
	p, e := syscall.UTF16PtrFromString(path)
	if e != nil {
		return false
	}
	var size uint32
	getSecurity.Call(uintptr(unsafe.Pointer(p)), 4, 0, 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		return false
	}
	buf := make([]byte, size)
	r, _, _ := getSecurity.Call(uintptr(unsafe.Pointer(p)), 4, uintptr(unsafe.Pointer(&buf[0])), uintptr(size), uintptr(unsafe.Pointer(&size)))
	if r == 0 {
		return false
	}
	var text uintptr
	r, _, _ = stringifySD.Call(uintptr(unsafe.Pointer(&buf[0])), 1, 4, uintptr(unsafe.Pointer(&text)), 0)
	if r == 0 {
		return false
	}
	defer kernel.NewProc("LocalFree").Call(text)
	var chars []uint16
	for i := uintptr(0); ; i++ {
		c := *(*uint16)(unsafe.Pointer(text + i*2))
		if c == 0 {
			break
		}
		chars = append(chars, c)
	}
	s := syscall.UTF16ToString(chars)
	sid, e := userSID()
	return e == nil && strings.HasPrefix(s, "D:P") && strings.Count(s, "(") == 1 && strings.Contains(s, "(A;;FA;;;"+sid+")")
}
