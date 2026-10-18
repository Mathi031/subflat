//go:build !windows

package main

// osLang: macOS and Linux terminals set LANG from the system language, so there
// is nothing beyond the environment to ask.
func osLang() string { return "en" }
