//go:build !windows

package main

import "os"

// enableVT: every other terminal understands ANSI escapes as is.
func enableVT(*os.File) bool { return true }
