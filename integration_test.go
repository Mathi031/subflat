package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Expected SRT for testdata/sample.ass with the default --min-gap and --lines 6.
// "·" stands for a padding line (padLine, checked byte by byte in
// TestFormatSRTLines), also after a sign alone.
var sampleSRT = strings.ReplaceAll(`1
00:00:01,000 --> 00:00:02,000
¿Dónde está el ñandú?

2
00:00:02,000 --> 00:00:04,000
En el corral, creo.
·
·
·
¿Dónde está el ñandú?
Granja Los Álamos

3
00:00:04,000 --> 00:00:05,000
En el corral, creo.
·
·
·
·
Granja Los Álamos

4
00:00:05,000 --> 00:00:05,960
Espera.
·
·
·
·
·

5
00:00:06,000 --> 00:00:08,000
Vamos a verlo.
Ahora mismo.

`, "·", padLine)

type mkvInfo struct {
	Container struct {
		Properties struct {
			Title string `json:"title"`
		} `json:"properties"`
	} `json:"container"`
	Chapters    []struct{} `json:"chapters"`
	Attachments []struct {
		FileName string `json:"file_name"`
	} `json:"attachments"`
	Tracks []track `json:"tracks"`
}

func TestIntegration(t *testing.T) {
	tl, err := findTools()
	if err != nil {
		t.Skip("MKVToolNix is not installed")
	}
	dir := t.TempDir()
	sample, _ := filepath.Abs(filepath.Join("testdata", "sample.ass"))
	write := func(name, s string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	chapters := write("chapters.txt", "CHAPTER01=00:00:00.000\nCHAPTER01NAME=Inicio\n")
	fre := write("fre.srt", "1\n00:00:01,000 --> 00:00:02,000\nBonjour\n\n")

	// Spaces, brackets and non-ASCII on purpose.
	src := filepath.Join(dir, "[Grupo] Prueba ñandú 01.mkv")
	mux := exec.Command(tl.mkvmerge, "-q", "-o", src, "--title", "Prueba", "--chapters", chapters,
		"--attachment-mime-type", "text/plain", "--attach-file", sample,
		"--language", "0:spa", "--track-name", "0:Español", "--default-track-flag", "0:yes",
		"--hearing-impaired-flag", "0:yes", "--original-flag", "0:no", sample,
		"--language", "0:fre", "--default-track-flag", "0:no", fre,
		"--language", "0:en", "--track-name", "0:English", "--default-track-flag", "0:no", "--forced-display-flag", "0:yes", sample)
	if out, err := mux.CombinedOutput(); err != nil {
		t.Fatalf("mkvmerge: %v\n%s", err, out)
	}
	before := hash(t, src)
	ctx := context.Background()

	type want struct {
		codec, lang, name string
		def, forced       bool
		extra             string
	}
	const spaFlags = "--hearing-impaired-flag 0:yes --original-flag 0:no"
	remux := func(t *testing.T, keepASS bool, wants []want) {
		out := filepath.Join(dir, t.Name())
		r := process(ctx, tl, config{dir: dir, out: out, minGap: 100, lines: 6, jobs: 1, keepASS: keepASS}, src)
		if r.status != done {
			t.Fatalf("status %d\n%s", r.status, strings.Join(r.lines, "\n"))
		}
		dst := filepath.Join(out, filepath.Base(src))
		info := identifyAll(t, tl, dst)
		if len(info.Tracks) != len(wants) {
			t.Fatalf("got %d tracks, want %d", len(info.Tracks), len(wants))
		}
		for i, w := range wants {
			tr := info.Tracks[i]
			p := tr.Properties
			got := want{p.CodecID, tr.lang(), p.Name, p.Default, p.Forced, strings.Join(tr.extraFlags(), " ")}
			if got != w {
				t.Errorf("track %d = %+v, want %+v", i, got, w)
			}
		}
		if info.Container.Properties.Title != "Prueba" || len(info.Chapters) != 1 ||
			len(info.Attachments) != 1 || info.Attachments[0].FileName != "sample.ass" {
			t.Errorf("title, chapters or attachments lost: %+v", info)
		}

		cmd := exec.Command(tl.mkvextract, dst, "tracks", "0:check.srt")
		cmd.Dir = out
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("mkvextract: %v\n%s", err, b)
		}
		b, _ := os.ReadFile(filepath.Join(out, "check.srt"))
		got := strings.ReplaceAll(strings.TrimPrefix(string(b), "\xef\xbb\xbf"), "\r\n", "\n")
		if got != sampleSRT {
			t.Errorf("muxed SRT:\n%q\nwant\n%q", got, sampleSRT)
		}

		if r := process(ctx, tl, config{dir: dir, out: out, minGap: 100, jobs: 1}, src); r.status != skipped {
			t.Errorf("second run status %d, want skipped", r.status)
		}
	}

	t.Run("replace", func(t *testing.T) {
		remux(t, false, []want{
			{"S_TEXT/UTF8", "es", "Español", true, false, spaFlags},
			{"S_TEXT/UTF8", "fr", "", false, false, ""},
			{"S_TEXT/UTF8", "en", "English", false, true, ""},
		})
	})
	t.Run("keep-ass", func(t *testing.T) {
		remux(t, true, []want{
			{"S_TEXT/UTF8", "es", "Español", true, false, spaFlags},
			{"S_TEXT/ASS", "es", "Español", false, false, spaFlags},
			{"S_TEXT/UTF8", "fr", "", false, false, ""},
			{"S_TEXT/UTF8", "en", "English", false, true, ""},
			{"S_TEXT/ASS", "en", "English", false, false, ""},
		})
	})
	t.Run("srt-only", func(t *testing.T) {
		out := filepath.Join(dir, "srt")
		if r := process(ctx, tl, config{dir: dir, out: out, minGap: 100, lines: 6, jobs: 1, srtOnly: true}, src); r.status != done {
			t.Fatalf("status %d\n%s", r.status, strings.Join(r.lines, "\n"))
		}
		b, err := os.ReadFile(filepath.Join(out, "[Grupo] Prueba ñandú 01.0.es.srt"))
		if err != nil {
			t.Fatal(err)
		}
		if want := strings.ReplaceAll(sampleSRT, "\n", "\r\n"); string(b) != want {
			t.Errorf("srt = %q", b)
		}
	})
	// The live progress path is only taken on a terminal: check the --gui-mode
	// parsing here, where there is none.
	t.Run("progress", func(t *testing.T) {
		var got []int
		cmd := exec.Command(tl.mkvmerge, "--gui-mode", "-o", filepath.Join(dir, "progress.mkv"), src)
		warns, err := runTool(cmd, func(p int) { got = append(got, p) })
		if err != nil || len(warns) != 0 {
			t.Fatalf("warns = %q, err = %v", warns, err)
		}
		if len(got) == 0 || got[len(got)-1] != 100 {
			t.Errorf("progress = %v, want it to end at 100", got)
		}
	})
	t.Run("dry-run writes nothing", func(t *testing.T) {
		out := filepath.Join(dir, "dry")
		if r := process(ctx, tl, config{dir: dir, out: out, minGap: 100, jobs: 1, dryRun: true}, src); r.status != done {
			t.Fatalf("status %d\n%s", r.status, strings.Join(r.lines, "\n"))
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Errorf("dry-run created %s", out)
		}
	})

	if hash(t, src) != before {
		t.Error("the original MKV changed")
	}
}

func identifyAll(t *testing.T, tl tools, path string) mkvInfo {
	out, err := exec.Command(tl.mkvmerge, "-J", path).Output()
	if err != nil {
		t.Fatalf("mkvmerge -J: %v", err)
	}
	var info mkvInfo
	if err := json.Unmarshal(out, &info); err != nil {
		t.Fatal(err)
	}
	return info
}

func hash(t *testing.T, path string) [32]byte {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(b)
}
