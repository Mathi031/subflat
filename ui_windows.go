//go:build windows

package main

import (
	"os"
	"syscall"
)

// enableVT turns on ANSI escape processing for the console behind f. Windows
// Terminal has it on already; the classic console needs the flag.
func enableVT(f *os.File) bool {
	const enableVirtualTerminalProcessing = 0x4
	h := syscall.Handle(f.Fd())
	var mode uint32
	if syscall.GetConsoleMode(h, &mode) != nil {
		return false
	}
	r, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleMode").Call(uintptr(h), uintptr(mode|enableVirtualTerminalProcessing))
	return r != 0
}
