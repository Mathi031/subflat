package main

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// findFFmpeg looks for an ffmpeg built with libass, which draws the subtitles.
func findFFmpeg() (string, error) {
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		install := "brew install ffmpeg"
		if runtime.GOOS == "windows" {
			install = "winget install Gyan.FFmpeg"
		}
		return "", fmt.Errorf(m.noFFmpeg, install)
	}
	out, err := exec.Command(p, "-hide_banner", "-filters").Output()
	if err != nil {
		return "", fmt.Errorf("%s -filters: %w", p, err)
	}
	if !strings.Contains(string(out), " subtitles ") {
		return "", errors.New(m.noLibass)
	}
	return p, nil
}

// pickEncoder returns the first encoder that encodes a test frame: for "auto",
// qsv (Intel Quick Sync) and then x265. When none does, the error carries the
// last line ffmpeg printed.
func pickEncoder(ctx context.Context, ffmpeg, enc string) (string, error) {
	cands := []string{enc}
	if enc == "auto" {
		cands = []string{"qsv", "x265"}
	}
	var err error
	for _, e := range cands {
		args := append([]string{"-hide_banner", "-v", "error", "-f", "lavfi", "-i", "color=s=256x256", "-frames:v", "1"},
			encoderArgs(e, 18)...)
		out, runErr := exec.CommandContext(ctx, ffmpeg, append(args, "-f", "null", "-")...).CombinedOutput()
		if runErr == nil {
			return e, nil
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		err = fmt.Errorf(m.noEncoder, e, cmp.Or(strings.TrimSpace(lines[len(lines)-1]), runErr.Error()))
	}
	return "", err
}

// encoderArgs are the ffmpeg video options: HEVC 10 bit, which every TV that
// plays HEVC accepts and which bands less in gradients than 8 bit.
func encoderArgs(enc string, quality int) []string {
	q := strconv.Itoa(quality)
	if enc == "qsv" {
		return []string{"-c:v", "hevc_qsv", "-profile:v", "main10", "-pix_fmt", "p010le", "-preset", "slow", "-global_quality", q}
	}
	return []string{"-c:v", "libx265", "-preset", "medium", "-crf", q, "-pix_fmt", "yuv420p10le", "-x265-params", "log-level=error"}
}

// burn draws <dir>/<ass> onto the first video track of src, with the fonts in
// <dir>/fonts, and writes only that video to dst. It runs inside dir so the
// filter never sees the path of src: brackets, quotes and drive colons would
// need escaping there. -copyts keeps the original timestamps, which the
// subtitles are timed against. With progress and the number of frames, it
// reports the percentage done, counting frames: with -copyts the out_time of
// -progress lags, by seconds on a short file. The errors ffmpeg prints on a run
// that succeeds (a damaged frame, a missing glyph) are returned as warnings, at
// most 5.
func (t tools) burn(ctx context.Context, dir, src, ass, dst, enc string, quality int, frames int64, progress func(int)) ([]string, error) {
	args := []string{"-nostdin", "-hide_banner", "-v", "error", "-nostats", "-progress", "pipe:1", "-y",
		"-copyts", "-i", src, "-map", "0:v:0", "-map_metadata", "-1", "-map_chapters", "-1",
		"-vf", "subtitles=" + ass + ":fontsdir=fonts"}
	args = append(args, encoderArgs(enc, quality)...)
	cmd := exec.CommandContext(ctx, t.ffmpeg, append(args, "-f", "matroska", dst)...)
	cmd.Dir = dir

	var errs []string
	out := &lineWriter{fn: func(l string) {
		v, ok := strings.CutPrefix(l, "frame=")
		if !ok || progress == nil || frames <= 0 {
			return
		}
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			progress(int(min(n*100/frames, 99)))
		}
	}}
	stderr := &lineWriter{fn: func(l string) { errs = append(errs, l) }}
	cmd.Stdout, cmd.Stderr = out, stderr
	err := cmd.Run()
	out.flush()
	stderr.flush()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: %v: %s", err, strings.Join(errs[max(len(errs)-5, 0):], " | "))
	}
	if len(errs) > 5 {
		errs = append(errs[:5], fmt.Sprintf("ffmpeg: (+%d)", len(errs)-5))
	}
	return errs, nil
}

// burnFile is process for --burn: pick the track, draw it onto the video and mux
// the audio back.
func burnFile(ctx context.Context, t tools, c config, src, dst, tmp string, info mkvInfo, ass []track, r *result, live bool) result {
	fail := func(err error) result { return r.end(failed, "%v", err) }
	vi := slices.IndexFunc(info.Tracks, func(tr track) bool { return tr.Type == "video" })
	if vi < 0 {
		return r.end(skipped, "%s", m.noVideo)
	}
	tr, again, err := c.pick.pick(ass)
	if err != nil {
		return r.end(skipped, "%v", err)
	}
	note := ""
	if again {
		note = " " + m.sameAsBefore
	}
	r.logf(m.burnTrack, tr.ID, tr.lang(), describe(tr), note)
	if c.dryRun {
		return r.end(done, m.wouldWrite, relName(c.out, dst))
	}

	if live {
		show("      " + m.extracting)
	}
	warns, err := t.extract(ctx, src, tmp, []int{tr.ID}, info.Attachments)
	if err != nil {
		return fail(err)
	}
	for _, w := range warns {
		r.warn(w)
	}
	var progress func(int)
	if live {
		show("      " + m.burning)
		progress = func(p int) { show(fmt.Sprintf("      %s %d%%", m.burning, p)) }
	}
	// The encoded video goes to the temp folder, usually a fast internal disk,
	// and only the final mux writes to the output, often a USB drive.
	video := filepath.Join(tmp, "video.mkv")
	warns, err = t.burn(ctx, tmp, src, fmt.Sprintf("%d.ass", tr.ID), video, c.encoder, c.quality,
		frameCount(info, info.Tracks[vi]), progress)
	if err != nil {
		return fail(err)
	}
	for _, w := range warns {
		r.warn(w)
	}
	if live {
		progress = func(p int) { show(fmt.Sprintf("      "+m.remuxing, p)) }
	}
	if err := writePart(r, dst, func(part string) ([]string, error) {
		return t.muxBurned(ctx, video, src, part, info.Tracks[vi], progress)
	}); err != nil {
		return fail(err)
	}
	return r.end(done, "%s", relName(c.out, dst))
}

// frameCount is the number of frames of the video track v: from mkvmerge's
// statistics tags, else from the container duration (which a longer track can
// stretch); 0 when unknown.
func frameCount(info mkvInfo, v track) int64 {
	if n, err := strconv.ParseInt(v.Properties.Frames, 10, 64); err == nil && n > 0 {
		return n
	}
	if d := v.Properties.FrameDur; d > 0 {
		return info.Container.Properties.Duration / d
	}
	return 0
}

// picker chooses the ASS track to burn, the same way for every file of a batch.
type picker struct {
	lang string                      // --burn-lang
	ask  func([]track) (track, bool) // nil without a terminal to ask on
	mu   sync.Mutex
	last *track // the previous answer, matched by language and name
}

// pick returns the track to burn and whether it repeats the previous answer.
// Several candidates and no way to ask is an error: the file is skipped.
func (p *picker) pick(ass []track) (track, bool, error) {
	cands := ass
	if p.lang != "" {
		cands = slices.DeleteFunc(slices.Clone(ass), func(tr track) bool { return !tr.hasLang(p.lang) })
		if len(cands) == 0 {
			return track{}, false, fmt.Errorf(m.noLangMatch, p.lang)
		}
	}
	if len(cands) == 1 {
		return cands[0], false, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.last != nil {
		same := slices.DeleteFunc(slices.Clone(cands), func(tr track) bool {
			return tr.lang() != p.last.lang() || tr.Properties.Name != p.last.Properties.Name
		})
		if len(same) == 1 {
			return same[0], true, nil
		}
	}
	if p.ask == nil {
		if p.lang != "" {
			return track{}, false, errors.New(m.manyASS)
		}
		return track{}, false, errors.New(m.manyASS + m.useBurnLang)
	}
	tr, ok := p.ask(cands)
	if !ok {
		return track{}, false, errors.New(m.noAnswer)
	}
	p.last = &tr
	return tr, false, nil
}

// hasLang matches the ISO 639-2 code (spa) or the IETF tag (es, es-419) of the
// track: "es" also takes "es-419".
func (t track) hasLang(l string) bool {
	p := t.Properties
	return strings.EqualFold(p.Language, l) || strings.EqualFold(p.LanguageIETF, l) ||
		len(p.LanguageIETF) > len(l) && strings.EqualFold(p.LanguageIETF[:len(l)+1], l+"-")
}

// stdinLines reads stdin once for the whole run, in the background.
var stdinLines = sync.OnceValue(func() <-chan string { return readLines(os.Stdin) })

// readLines sends each line of r, closing the channel at the end of the input.
// Reading in a goroutine lets askTrack give up on Ctrl+C: on Linux and macOS a
// blocked read survives the signal, which signal.NotifyContext catches.
func readLines(r io.Reader) <-chan string {
	ch := make(chan string)
	go func() {
		defer close(ch)
		s := bufio.NewScanner(r)
		for s.Scan() {
			ch <- s.Text()
		}
	}()
	return ch
}

// askTrack lists the candidates and reads numbers until one is valid. The end of
// the input or Ctrl+C means no answer.
func askTrack(ctx context.Context, lines <-chan string, cands []track) (track, bool) {
	fmt.Println("      " + m.askTrack)
	for i, tr := range cands {
		fmt.Printf("        %d) %s %d  %s%s\n", i+1, m.track, tr.ID, tr.lang(), describe(tr))
	}
	for {
		fmt.Print("      " + m.askNumber)
		select {
		case l, ok := <-lines:
			if !ok {
				fmt.Println()
				return track{}, false
			}
			if n, err := strconv.Atoi(strings.TrimSpace(l)); err == nil && n >= 1 && n <= len(cands) {
				return cands[n-1], true
			}
		case <-ctx.Done():
			fmt.Println()
			return track{}, false
		}
	}
}
