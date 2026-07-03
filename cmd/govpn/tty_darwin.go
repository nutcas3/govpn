//go:build darwin

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
		syscall.TIOCGETA,
		uintptr(unsafe.Pointer(&t)),
	)
	return errno == 0
}
