package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
Granja Los Álamos
·
·
·
¿Dónde está el ñandú?

3
00:00:04,000 --> 00:00:05,000
En el corral, creo.
Granja Los Álamos
·
·
·
·

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

type fileInfo struct {
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

func identifyAll(t *testing.T, tl tools, path string) fileInfo {
	out, err := exec.Command(tl.mkvmerge, "-J", path).Output()
	if err != nil {
		t.Fatalf("mkvmerge -J: %v", err)
	}
	var info fileInfo
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

func TestBurnIntegration(t *testing.T) {
	tl, err := findTools()
	if err != nil {
		t.Skip("MKVToolNix is not installed")
	}
	if tl.ffmpeg, err = findFFmpeg(); err != nil {
		t.Skip("ffmpeg with libass is not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is not installed")
	}
	dir := t.TempDir()
	sample, _ := filepath.Abs(filepath.Join("testdata", "sample.ass"))
	run := func(name string, args ...string) string {
		out, err := exec.Command(name, args...).Output()
		if err != nil {
			t.Fatalf("%s %q: %v", filepath.Base(name), args, err)
		}
		return string(out)
	}
	video := filepath.Join(dir, "video.mkv")
	run(tl.ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=640x360:r=24000/1001:d=3",
		"-f", "lavfi", "-i", "sine=d=3", "-c:v", "ffv1", "-c:a", "flac", video)
	chapters := filepath.Join(dir, "chapters.txt")
	if err := os.WriteFile(chapters, []byte("CHAPTER01=00:00:00.000\nCHAPTER01NAME=Inicio\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Spaces, brackets and non-ASCII on purpose: ffmpeg must never see this name in a filter.
	src := filepath.Join(dir, "[Grupo] Quemar ñandú 01.mkv")
	run(tl.mkvmerge, "-q", "-o", src, "--title", "Prueba", "--chapters", chapters,
		"--attachment-mime-type", "text/plain", "--attach-file", sample, "--language", "0:jpn", video,
		"--language", "0:spa", "--track-name", "0:Español", sample, "--language", "0:en", sample)
	before := hash(t, src)
	ctx := context.Background()

	// The progress of burnFile counts frames from the JSON of mkvmerge -J.
	in, err := tl.identify(ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	if n := frameCount(in, in.Tracks[0]); n != 72 {
		t.Errorf("frameCount = %d, want 72", n)
	}

	out := filepath.Join(dir, "tv")
	c := config{dir: dir, out: out, jobs: 1, burn: true, encoder: "x265", quality: 30, pick: &picker{lang: "es"}}
	if r := process(ctx, tl, c, src); r.status != done {
		t.Fatalf("status %d: %s\n%s", r.status, r.text, strings.Join(r.lines, "\n"))
	}
	dst := filepath.Join(out, filepath.Base(src))
	info := identifyAll(t, tl, dst)
	var got []string
	for _, tr := range info.Tracks {
		got = append(got, tr.Properties.CodecID+" "+tr.lang())
	}
	if g := strings.Join(got, ", "); g != "V_MPEGH/ISO/HEVC ja, A_FLAC und" {
		t.Errorf("tracks = %s, want the HEVC video in the original's language and the audio only", g)
	}
	if info.Container.Properties.Title != "Prueba" || len(info.Chapters) != 1 || len(info.Attachments) != 0 {
		t.Errorf("title or chapters lost, or attachments kept: %+v", info)
	}

	frames := func(p string) string {
		return run(ffprobe, "-v", "error", "-select_streams", "v:0", "-count_frames",
			"-show_entries", "stream=nb_read_frames,r_frame_rate", "-of", "csv=p=0", p)
	}
	if a, b := frames(src), frames(dst); a != b {
		t.Errorf("frames and rate: original %q, burned %q", a, b)
	}

	// The text is drawn from 1 s on: before, the burned frame only differs by the
	// encoding; after, the subtitle adds a much larger error.
	stats := run(tl.ffmpeg, "-v", "error", "-i", src, "-i", dst, "-lavfi",
		"[0:v]format=yuv420p10le[a];[1:v]format=yuv420p10le[b];[a][b]psnr=stats_file=-", "-f", "null", "-")
	mse := map[string]float64{}
	for _, l := range strings.Split(stats, "\n") {
		var n int
		var v float64
		if _, err := fmt.Sscanf(l, "n:%d mse_avg:%f", &n, &v); err == nil && (n == 13 || n == 37) {
			mse[fmt.Sprint(n)] = v
		}
	}
	if len(mse) != 2 || mse["37"] < 4*mse["13"] {
		t.Errorf("MSE at 0.5 s %v, at 1.5 s %v: the subtitle is not drawn", mse["13"], mse["37"])
	}

	if r := process(ctx, tl, c, src); r.status != skipped {
		t.Errorf("second run status %d, want skipped", r.status)
	}

	// The fonts land where the filter's fontsdir=fonts looks for them.
	t.Run("fonts extracted", func(t *testing.T) {
		work := t.TempDir()
		if _, err := tl.extract(ctx, src, work, []int{1}, in.Attachments); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{"1.ass", filepath.Join("fonts", fmt.Sprintf("%d.ass", in.Attachments[0].ID))} {
			if _, err := os.Stat(filepath.Join(work, p)); err != nil {
				t.Error(err)
			}
		}
	})

	// Without attachments there is no fonts folder: libass falls back to the
	// system fonts. Check the -progress parsing here too, where there is no
	// terminal.
	t.Run("progress, no fonts", func(t *testing.T) {
		work := t.TempDir()
		b, _ := os.ReadFile(sample)
		if err := os.WriteFile(filepath.Join(work, "1.ass"), b, 0o644); err != nil {
			t.Fatal(err)
		}
		var got []int
		_, err := tl.burn(ctx, work, src, "1.ass", filepath.Join(work, "v.mkv"), "x265", 30, 72, func(p int) { got = append(got, p) })
		if err != nil {
			t.Fatal(err)
		}
		if len(got) == 0 || got[len(got)-1] != 99 {
			t.Errorf("progress = %v, want it to end at 99", got)
		}
	})

	if hash(t, src) != before {
		t.Error("the original MKV changed")
	}
}
