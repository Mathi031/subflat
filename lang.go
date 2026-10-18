package main

import (
	"cmp"
	"fmt"
	"os"
	"strings"
)

// setLang picks the message language: the --lang value, then SUBFLAT_LANG, then
// the system language, else English.
func setLang(flagValue string) error {
	switch v := cmp.Or(flagValue, os.Getenv("SUBFLAT_LANG"), systemLang()); v {
	case "en":
		m = en
	case "es":
		m = es
	default:
		return fmt.Errorf(m.errLang, v)
	}
	return nil
}

// systemLang reads the POSIX locale variables and, without them, asks the OS.
func systemLang() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			if strings.HasPrefix(strings.ToLower(v), "es") {
				return "es"
			}
			return "en"
		}
	}
	return osLang()
}
