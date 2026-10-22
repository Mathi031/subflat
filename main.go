// subflat converts the ASS/SSA tracks of MKV files to SRT, merging simultaneous
// events, so TVs that only show one subtitle at a time show every line.
package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"
)

type config struct {
	dir, file, out                             string // file is set when the target is a single MKV
	recursive, keepASS, srtOnly, dryRun, force bool
	exclude                                    *regexp.Regexp
	minGap, jobs, lines                        int
}

var errVersion = errors.New("version")

// newFlags registers every option. The help is written by hand per language
// (msg.go), so the descriptions stay empty and flag itself prints nothing.
func newFlags(c *config, exclude, lang *string, version *bool) *flag.FlagSet {
	f := flag.NewFlagSet("subflat", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.StringVar(&c.out, "out", "", "")
	f.BoolVar(&c.recursive, "recursive", false, "")
	f.BoolVar(&c.keepASS, "keep-ass", false, "")
	f.StringVar(exclude, "exclude-style", "", "")
	f.IntVar(&c.minGap, "min-gap", 100, "")
	f.IntVar(&c.lines, "lines", 17, "")
	f.BoolVar(&c.srtOnly, "srt-only", false, "")
	f.BoolVar(&c.dryRun, "dry-run", false, "")
	f.BoolVar(&c.force, "force", false, "")
	f.IntVar(&c.jobs, "jobs", 1, "")
	f.StringVar(lang, "lang", "", "")
	f.BoolVar(version, "version", false, "")
	return f
}

func parseArgs(args []string) (config, error) {
	var c config
	// --lang decides the language of the help and of every error below, so it
	// is read before the flags are parsed.
	if err := setLang(langArg(args)); err != nil {
		return c, err
	}
	var exclude, lang string
	var showVersion bool
	flags := newFlags(&c, &exclude, &lang, &showVersion)

	// flag stops at the first positional argument: parse again after each one so
	// options work before and after the target.
	var pos []string
	for {
		if err := flags.Parse(args); err != nil {
			return c, err
		}
		if flags.NArg() == 0 {
			break
		}
		pos = append(pos, flags.Arg(0))
		args = flags.Args()[1:]
	}
	switch {
	case showVersion:
		return c, errVersion
	case len(pos) == 0:
		return c, flag.ErrHelp
	case len(pos) > 1:
		return c, errors.New(m.errOneTarget)
	case c.jobs < 1:
		return c, errors.New(m.errJobs)
	case c.minGap < 0:
		return c, errors.New(m.errMinGap)
	case c.lines < 0:
		return c, errors.New(m.errLines)
	}
	// Absolute paths: mkvextract runs inside the temp dir, and mkvmerge reads a
	// relative name starting with @ as an options file.
	var err error
	if c.dir, err = filepath.Abs(pos[0]); err != nil {
		return c, err
	}
	// A single MKV works too: its folder is the base for --out and relative names.
	if fi, err := os.Stat(c.dir); err == nil && !fi.IsDir() {
		c.file, c.dir = c.dir, filepath.Dir(c.dir)
	}
	if c.out == "" {
		c.out = filepath.Join(c.dir, "tv")
	}
	if c.out, err = filepath.Abs(c.out); err != nil {
		return c, err
	}
	if exclude != "" {
		re, err := regexp.Compile(exclude)
		if err != nil {
			return c, fmt.Errorf("--exclude-style: %w", err)
		}
		c.exclude = re
	}
	return c, nil
}

// langArg finds the --lang value in args, with or without "=".
func langArg(args []string) string {
	for i, a := range args {
		k, v, has := strings.Cut(a, "=")
		if k != "-lang" && k != "--lang" {
			continue
		}
		if !has && i+1 < len(args) {
			v = args[i+1]
		}
		return v
	}
	return ""
}

// version comes from the module version (go install ...@v1.0.0) or, with Go
// 1.24+, from the git tag of the commit being built.
func version() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return strings.TrimPrefix(bi.Main.Version, "v")
	}
	return "dev"
}

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	initUI()
	c, err := parseArgs(args)
	switch {
	case errors.Is(err, flag.ErrHelp):
		fmt.Printf(boldHeadings(m.help), version())
		return 0
	case errors.Is(err, errVersion):
		fmt.Println("subflat", version())
		return 0
	case err != nil:
		return usageError(err)
	}
	t, err := findTools()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %v\n", paint(red, "error:"), err)
		return 2
	}
	files, err := findMKVs(c)
	if err != nil {
		return usageError(err)
	}
	fmt.Printf("%s %s\n%s  ->  %s\n", paint(bold, "subflat"), version(), cmp.Or(c.file, c.dir), c.out)
	if c.dryRun {
		fmt.Println(paint(yellow, m.dryRun))
	}
	fmt.Printf("%s: %d\n\n", m.mkvFiles, len(files))
	if len(files) == 0 {
		return 0
	}
	start := time.Now()

	// Ctrl+C cancels ctx: running mkvmerge is killed and the deferred cleanups run.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	type job struct {
		i int
		f string
	}
	in := make(chan job)
	results := make(chan result)
	var wg sync.WaitGroup
	var mu sync.Mutex // one result block at a time when files run in parallel
	for range c.jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range in {
				rel, _ := filepath.Rel(c.dir, j.f)
				fmt.Printf("[%d/%d] %s\n", j.i+1, len(files), paint(bold, rel))
				r := process(ctx, t, c, j.f)
				mu.Lock()
				for _, l := range r.lines {
					fmt.Println(l)
				}
				fmt.Printf("  %s  %s  (%s)\n\n", r.marker(), r.text, took(r.took))
				mu.Unlock()
				results <- r
			}
		}()
	}
	go func() {
		defer close(in)
		for i, f := range files {
			select {
			case in <- job{i, f}:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { wg.Wait(); close(results) }()

	var count [3]int
	warnings := 0
	for r := range results {
		count[r.status]++
		warnings += r.warnings
	}
	fmt.Printf(m.summary+"  (%s)\n", num(count[done], green), num(count[skipped], yellow),
		num(count[failed], red), num(warnings, yellow), took(time.Since(start)))
	if ctx.Err() != nil {
		fmt.Println(paint(red, m.interrupted))
		return 1
	}
	if count[failed] > 0 {
		return 1
	}
	return 0
}

func usageError(err error) int {
	fmt.Fprintf(os.Stderr, "%s %v\n%s\n", paint(red, "error:"), err, m.usageHint)
	return 2
}

// num paints a count only when there is something to notice.
func num(n int, code string) string {
	if n == 0 {
		return "0"
	}
	return paint(code, fmt.Sprint(n))
}

func took(d time.Duration) string { return d.Round(100 * time.Millisecond).String() }

func findMKVs(c config) ([]string, error) {
	if c.file != "" {
		if !strings.EqualFold(filepath.Ext(c.file), ".mkv") {
			return nil, fmt.Errorf(m.notTarget, c.file)
		}
		return []string{c.file}, nil
	}
	info, err := os.Stat(c.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf(m.notFound, c.dir)
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf(m.notTarget, c.dir)
	}
	var files []string
	err = filepath.WalkDir(c.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// The default output dir lives inside the input dir: never read it back.
			if p != c.dir && (!c.recursive || samePath(p, c.out)) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(p), ".mkv") {
			files = append(files, p)
		}
		return nil
	})
	return files, err
}

const (
	done = iota
	skipped
	failed
)

type result struct {
	status   int
	lines    []string // detail lines, already indented and painted
	text     string   // what the status line says
	warnings int
	took     time.Duration
}

func (r *result) logf(format string, a ...any) {
	r.lines = append(r.lines, paint(dim, "      "+fmt.Sprintf(format, a...)))
}

func (r *result) warn(msg string) {
	r.warnings++
	r.lines = append(r.lines, paint(yellow, "      "+m.warning+" "+msg))
}

func (r *result) end(status int, format string, a ...any) result {
	r.status = status
	r.text = fmt.Sprintf(format, a...)
	return *r
}

func (r result) marker() string {
	switch r.status {
	case skipped:
		return paint(yellow, m.skipped)
	case failed:
		return paint(red, m.failed)
	}
	return paint(green, "ok")
}

// process handles one MKV. Any error, even a panic, stays in its result so the
// batch goes on.
func process(ctx context.Context, t tools, c config, src string) (r result) {
	start := time.Now()
	defer func() {
		r.took = time.Since(start)
		if p := recover(); p != nil {
			r.status = failed
			r.text = fmt.Sprintf(m.internal, p)
		}
	}()
	fail := func(err error) result { return r.end(failed, "%v", err) }
	// Live progress only makes sense one file at a time, on a terminal.
	live := c.jobs == 1 && tty && !c.dryRun
	if live {
		defer show("")
	}

	rel, err := filepath.Rel(c.dir, src)
	if err != nil {
		return fail(err)
	}
	dst := filepath.Join(c.out, rel)
	if samePath(dst, src) {
		return fail(errors.New(m.sameAsSrc))
	}

	tracks, err := t.identify(ctx, src)
	if err != nil {
		return fail(err)
	}
	var ass []track
	for _, tr := range tracks {
		note := ""
		if tr.isASS() {
			ass = append(ass, tr)
			note = "  " + m.toSRT
		}
		r.logf("%s %d  %-9s %-16s %s%s%s", m.track, tr.ID, tr.Type, tr.Properties.CodecID, tr.lang(), describe(tr), note)
	}
	if len(ass) == 0 {
		return r.end(skipped, "%s", m.noASS)
	}

	targets := []string{dst}
	if c.srtOnly {
		targets = nil
		for _, tr := range ass {
			targets = append(targets, srtPath(dst, tr))
		}
	}
	for _, p := range targets {
		if _, err := os.Stat(p); err == nil && !c.force {
			if !c.dryRun {
				return r.end(skipped, m.exists, relName(c.out, p))
			}
			r.logf(m.existsDry, relName(c.out, p))
		}
	}

	tmp, err := os.MkdirTemp("", "subflat-")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(tmp)

	var ids []int
	for _, tr := range ass {
		ids = append(ids, tr.ID)
	}
	if live {
		show("      " + m.extracting)
	}
	warns, err := t.extract(ctx, src, tmp, ids)
	if err != nil {
		return fail(err)
	}
	for _, w := range warns {
		r.warn(w)
	}

	srts := map[int]string{}
	outs := []string{dst}
	if c.srtOnly {
		outs = nil
	}
	for _, tr := range ass {
		data, err := os.ReadFile(filepath.Join(tmp, fmt.Sprintf("%d.ass", tr.ID)))
		if err != nil {
			return fail(err)
		}
		s := ParseASS(string(data))
		evs := WithoutStyles(s.Events, c.exclude)
		blocks := Merge(evs, c.minGap)
		r.logf(m.events, tr.ID, len(evs), len(blocks))
		if s.Karaoke > 0 {
			r.logf(m.karaoke, tr.ID, s.Karaoke, s.Restored)
		}
		r.logf("  "+m.styles, styleSummary(s.Styles, c.exclude))
		if s.Bad > 0 {
			r.warn(fmt.Sprintf(m.badLines, tr.ID, s.Bad))
		}
		for _, w := range Warnings(blocks) {
			r.warn(fmt.Sprintf("%s %d, %s", m.track, tr.ID, w))
		}
		if len(blocks) == 0 {
			r.warn(fmt.Sprintf(m.emptyTrack, tr.ID))
			continue
		}
		p := filepath.Join(tmp, fmt.Sprintf("%d.srt", tr.ID))
		if c.srtOnly {
			p = srtPath(dst, tr)
			outs = append(outs, p)
		}
		srts[tr.ID] = p
		if c.dryRun {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return fail(err)
		}
		if err := os.WriteFile(p, []byte(FormatSRT(blocks, c.lines)), 0o644); err != nil {
			return fail(err)
		}
	}

	var names []string
	for _, p := range outs {
		names = append(names, relName(c.out, p))
	}
	written := strings.Join(names, ", ")
	switch {
	case len(srts) == 0:
		return r.end(skipped, "%s", m.noDialogue)
	case c.dryRun:
		return r.end(done, m.wouldWrite, written)
	case c.srtOnly:
		return r.end(done, "%s", written)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fail(err)
	}
	// Write to .part and rename at the end: an interrupted remux never leaves a
	// destination that the next run would skip as already done.
	part := dst + ".part"
	defer os.Remove(part)
	var progress func(int)
	if live {
		progress = func(p int) { show(fmt.Sprintf("      "+m.remuxing, p)) }
	}
	warns, err = t.remux(ctx, src, part, tracks, srts, c.keepASS, progress)
	if err != nil {
		return fail(err)
	}
	for _, w := range warns {
		r.warn(w)
	}
	if err := os.Rename(part, dst); err != nil {
		return fail(err)
	}
	return r.end(done, "%s", written)
}

func describe(tr track) string {
	p := tr.Properties
	s := ""
	if p.Name != "" {
		s += fmt.Sprintf(" %q", p.Name)
	}
	if p.Default {
		s += " default"
	}
	if p.Forced {
		s += " forced"
	}
	return s
}

func srtPath(dst string, tr track) string {
	return strings.TrimSuffix(dst, filepath.Ext(dst)) + fmt.Sprintf(".%d.%s.srt", tr.ID, tr.lang())
}

// relName shortens an output path to its name under the output folder.
func relName(base, p string) string {
	if rel, err := filepath.Rel(base, p); err == nil {
		return rel
	}
	return p
}

func styleSummary(styles map[string]int, exclude *regexp.Regexp) string {
	names := slices.Collect(maps.Keys(styles))
	slices.SortFunc(names, func(a, b string) int {
		return cmp.Or(cmp.Compare(styles[b], styles[a]), strings.Compare(a, b))
	})
	var parts []string
	for _, n := range names {
		s := fmt.Sprintf("%s %d", n, styles[n])
		if exclude != nil && exclude.MatchString(n) {
			s += " " + m.excluded
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

// samePath ignores case: Windows and macOS file systems do by default. On a
// case-sensitive one it can only refuse a valid path, never overwrite the original.
func samePath(a, b string) bool {
	a, errA := filepath.Abs(a)
	b, errB := filepath.Abs(b)
	return errA == nil && errB == nil && strings.EqualFold(a, b)
}
