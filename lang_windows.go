//go:build windows

package main

import "syscall"

// osLang asks Windows for the user's display language. The low 10 bits of the
// LANGID are the primary language; 0x0A is Spanish in every region.
func osLang() string {
	id, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage").Call()
	if id&0x3ff == 0x0a {
		return "es"
	}
	return "en"
}
