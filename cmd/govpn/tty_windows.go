//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode = kernel32.NewProc("GetConsoleMode")
)

func isTerminal() bool {
	var mode uint32
	r, _, _ := getConsoleMode.Call(os.Stderr.Fd(), uintptr(unsafe.Pointer(&mode)))
	return r != 0
}
