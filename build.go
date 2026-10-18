//go:build ignore

// Builds subflat for every supported platform into dist/, from any OS:
//
//	go run build.go
package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	for _, t := range [][2]string{{"windows", "amd64"}, {"darwin", "arm64"}, {"darwin", "amd64"}} {
		out := fmt.Sprintf("dist/subflat-%s-%s", t[0], t[1])
		if t[0] == "windows" {
			out += ".exe"
		}
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", out, ".")
		cmd.Env = append(os.Environ(), "GOOS="+t[0], "GOARCH="+t[1], "CGO_ENABLED=0")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, out+":", err)
			os.Exit(1)
		}
		fmt.Println(out)
	}
}
