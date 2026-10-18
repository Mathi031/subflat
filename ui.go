package main

import (
	"fmt"
	"os"
	"strings"
)

const (
	bold   = "1"
	dim    = "2"
	red    = "31"
	green  = "32"
	yellow = "33"
)

// tty is true when stdout is a terminal: progress is then rewritten in place.
// color adds ANSI colors on top, unless NO_COLOR or TERM=dumb asks for plain text.
var tty, color bool

func initUI() {
	fi, err := os.Stdout.Stat()
	tty = err == nil && fi.Mode()&os.ModeCharDevice != 0
	color = tty && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" && enableVT(os.Stdout)
}

func paint(code, s string) string {
	if !color {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// show rewrites the current line in place; show("") clears it.
func show(s string) { fmt.Printf("\r%-70s\r", s) }

// boldHeadings paints the all-caps section titles of the help.
func boldHeadings(help string) string {
	ls := strings.Split(help, "\n")
	for i, l := range ls {
		if l != "" && !strings.HasPrefix(l, " ") && l == strings.ToUpper(l) {
			ls[i] = paint(bold, l)
		}
	}
	return strings.Join(ls, "\n")
}
