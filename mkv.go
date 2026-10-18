package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type tools struct{ mkvmerge, mkvextract string }

type track struct {
	ID         int    `json:"id"`
	Type       string `json:"type"`
	Properties struct {
		CodecID      string `json:"codec_id"`
		Language     string `json:"language"`
		LanguageIETF string `json:"language_ietf"`
		Name         string `json:"track_name"`
		Default      bool   `json:"default_track"`
		Forced       bool   `json:"forced_track"`
		// Only present when the file sets them.
		HearingImpaired  *bool `json:"flag_hearing_impaired"`
		VisualImpaired   *bool `json:"flag_visual_impaired"`
		TextDescriptions *bool `json:"flag_text_descriptions"`
		Original         *bool `json:"flag_original"`
		Commentary       *bool `json:"flag_commentary"`
	} `json:"properties"`
}

// extraFlags returns the mkvmerge options that copy the flags beyond default and
// forced to track 0 of the next input, for the ones the track sets.
func (t track) extraFlags() []string {
	p := t.Properties
	var args []string
	for _, f := range []struct {
		opt string
		v   *bool
	}{
		{"--hearing-impaired-flag", p.HearingImpaired},
		{"--visual-impaired-flag", p.VisualImpaired},
		{"--text-descriptions-flag", p.TextDescriptions},
		{"--original-flag", p.Original},
		{"--commentary-flag", p.Commentary},
	} {
		if f.v != nil {
			args = append(args, f.opt, "0:"+yesNo(*f.v))
		}
	}
	return args
}

func (t track) isASS() bool {
	c := t.Properties.CodecID
	return t.Type == "subtitles" && (c == "S_TEXT/ASS" || c == "S_TEXT/SSA")
}

func (t track) lang() string {
	return cmp.Or(t.Properties.LanguageIETF, t.Properties.Language, "und")
}

// findTools looks for MKVToolNix in PATH and, on Windows, in its default install folder.
func findTools() (tools, error) {
	find := func(name string) string {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
		if runtime.GOOS == "windows" {
			p := filepath.Join(cmp.Or(os.Getenv("ProgramFiles"), `C:\Program Files`), "MKVToolNix", name+".exe")
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		return ""
	}
	t := tools{find("mkvmerge"), find("mkvextract")}
	if t.mkvmerge == "" || t.mkvextract == "" {
		install := "brew install mkvtoolnix"
		if runtime.GOOS == "windows" {
			install = "winget install MoritzBunkus.MKVToolNix"
		}
		return t, fmt.Errorf(m.noTools, install)
	}
	return t, nil
}

func (t tools) identify(ctx context.Context, path string) ([]track, error) {
	out, runErr := exec.CommandContext(ctx, t.mkvmerge, "-J", path).Output()
	var info struct {
		Container struct{ Recognized bool } `json:"container"`
		Errors    []string                  `json:"errors"`
		Tracks    []track                   `json:"tracks"`
	}
	// The JSON carries the reason even when mkvmerge exits with an error.
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("mkvmerge -J: %w", cmp.Or(runErr, err))
	}
	if len(info.Errors) > 0 {
		return nil, errors.New(strings.Join(info.Errors, "; "))
	}
	if runErr != nil {
		return nil, fmt.Errorf("mkvmerge -J: %w", runErr)
	}
	if !info.Container.Recognized {
		return nil, errors.New(m.unknownFormat)
	}
	return info.Tracks, nil
}

// extract writes each track to <dir>/<id>.ass. It runs inside dir with relative
// names so the "id:file" spec never meets the colon of a Windows drive.
func (t tools) extract(ctx context.Context, src, dir string, ids []int) ([]string, error) {
	args := []string{src, "tracks", "-q"}
	for _, id := range ids {
		args = append(args, fmt.Sprintf("%d:%d.ass", id, id))
	}
	cmd := exec.CommandContext(ctx, t.mkvextract, args...)
	cmd.Dir = dir
	return runTool(cmd, nil)
}

// remux writes dst with every track of src, each ASS replaced by (or, with keepASS,
// followed by) its SRT in srts, keeping language, name, flags and order. With
// progress, mkvmerge reports its percentage through it.
func (t tools) remux(ctx context.Context, src, dst string, tracks []track, srts map[int]string, keepASS bool, progress func(int)) ([]string, error) {
	var assIDs, order, inputs []string
	file := 1
	for _, tr := range tracks {
		id := strconv.Itoa(tr.ID)
		srt, ok := srts[tr.ID]
		if !ok {
			order = append(order, "0:"+id)
			continue
		}
		assIDs = append(assIDs, id)
		p := tr.Properties
		inputs = append(inputs, "--sub-charset", "0:UTF-8", "--language", "0:"+tr.lang(),
			"--default-track-flag", "0:"+yesNo(p.Default), "--forced-display-flag", "0:"+yesNo(p.Forced))
		inputs = append(inputs, tr.extraFlags()...)
		if p.Name != "" {
			inputs = append(inputs, "--track-name", "0:"+p.Name)
		}
		inputs = append(inputs, srt)
		order = append(order, fmt.Sprintf("%d:0", file))
		if keepASS {
			order = append(order, "0:"+id)
		}
		file++
	}

	args := []string{"-q", "-o", dst, "--track-order", strings.Join(order, ",")}
	if progress != nil {
		args[0] = "--gui-mode"
	}
	if keepASS {
		for _, id := range assIDs {
			args = append(args, "--default-track-flag", id+":no", "--forced-display-flag", id+":no")
		}
	} else {
		args = append(args, "--subtitle-tracks", "!"+strings.Join(assIDs, ","))
	}
	args = append(args, src)
	return runTool(exec.CommandContext(ctx, t.mkvmerge, append(args, inputs...)...), progress)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// runTool returns MKVToolNix warnings: exit 1 means a valid output with warnings,
// 2 an error. With -q the tool prints only warnings and errors. With progress the
// command runs in --gui-mode instead: "#GUI#progress N%" lines feed progress,
// "#GUI#warning" and "#GUI#error" lines are kept, the status chatter is dropped.
func runTool(cmd *exec.Cmd, progress func(int)) ([]string, error) {
	var lines []string
	lw := &lineWriter{fn: func(l string) {
		if progress == nil {
			lines = append(lines, l)
			return
		}
		msg, isGUI := strings.CutPrefix(l, "#GUI#")
		kind, msg, _ := strings.Cut(msg, " ")
		switch {
		case !isGUI:
		case kind == "progress":
			if n, err := strconv.Atoi(strings.TrimSuffix(msg, "%")); err == nil {
				progress(n)
			}
		case kind == "warning" || kind == "error":
			lines = append(lines, msg)
		}
	}}
	cmd.Stdout, cmd.Stderr = lw, lw
	err := cmd.Run()
	lw.flush()
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return lines, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %v: %s", filepath.Base(cmd.Path), err, strings.Join(lines, " | "))
	}
	return lines, nil
}

// lineWriter hands complete, trimmed, non-empty lines to fn as they arrive.
type lineWriter struct {
	buf []byte
	fn  func(string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexAny(w.buf, "\r\n")
		if i < 0 {
			return len(p), nil
		}
		w.emit(w.buf[:i])
		w.buf = w.buf[i+1:]
	}
}

func (w *lineWriter) flush() {
	w.emit(w.buf)
	w.buf = nil
}

func (w *lineWriter) emit(b []byte) {
	if l := strings.TrimSpace(string(b)); l != "" {
		w.fn(l)
	}
}
