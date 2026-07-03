//go:build linux || darwin

package main

import (
	"os"
	"syscall"
	"unsafe"
)

func isTerminal() bool {
	var t syscall.Termios
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		os.Stderr.Fd(),
		syscall.TCGETS,
		uintptr(unsafe.Pointer(&t)),
	)
	return errno == 0
}
