package main

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Event is a Dialogue line with its text already cleaned.
type Event struct {
	Start, End int // ms
	Style      string
	Text       string
	Top        bool // alignment 7-9
	Order      int  // position in the file
}

// Script is what the merge needs from an ASS/SSA file.
type Script struct {
	Events []Event        // Dialogue lines with text, drawings already dropped
	Styles map[string]int // Dialogue lines per style, before dropping anything
	Bad    int            // malformed Dialogue lines skipped
}

// Block is one SRT cue: the union of the events active in [Start, End). The first
// Top texts come from top-aligned events (signs, a second speaker), the rest is
// dialogue. No position tags: the Tizen player shows {\an8} as text.
type Block struct {
	Start, End int
	Texts      []string
	Top        int
}

func (b Block) Text() string { return strings.Join(b.Texts, "\n") }

var (
	overrideRe = regexp.MustCompile(`\{[^}]*\}`)
	// "an" goes first so \an8 is not read as \a; \alpha, \pos and \pbo have no digit right after.
	tagRe        = regexp.MustCompile(`\\(an|a|p)(\d+)`)
	escapes      = strings.NewReplacer(`\N`, "\n", `\n`, "\n", `\h`, " ")
	eventsFormat = []string{"layer", "start", "end", "style", "name", "marginl", "marginr", "marginv", "effect", "text"}
)

// ParseASS reads the styles and Dialogue events of an ASS or SSA script.
// Columns follow each section's Format: line.
func ParseASS(src string) Script {
	s := Script{Styles: map[string]int{}}
	styleAlign := map[string]int{}
	defaultAlign := 2
	var section string
	var styleFmt, eventFmt map[string]int
	type raw struct {
		ev    Event
		align int // override from \an or \a, 0 = use the style
	}
	var raws []raw

	for _, line := range strings.Split(strings.TrimPrefix(src, "\xef\xbb\xbf"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(line)
			continue
		}
		key, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		switch {
		case strings.Contains(section, "styles") && key == "Format":
			styleFmt = columns(rest)
		case strings.Contains(section, "styles") && key == "Style":
			name, align, ok := parseStyle(rest, styleFmt, section == "[v4 styles]")
			if !ok {
				continue
			}
			styleAlign[name] = align // a repeated name overrides, as in libass
			if strings.EqualFold(name, "Default") {
				defaultAlign = align
			}
		case section == "[events]" && key == "Format":
			eventFmt = columns(rest)
		case section == "[events]" && key == "Dialogue":
			if eventFmt == nil {
				eventFmt = columns(strings.Join(eventsFormat, ","))
			}
			f := strings.SplitN(rest, ",", len(eventFmt))
			if len(f) < len(eventFmt) {
				s.Bad++
				continue
			}
			get := func(k string) string {
				if i, ok := eventFmt[k]; ok {
					return f[i]
				}
				return ""
			}
			start, err1 := parseTime(get("start"))
			end, err2 := parseTime(get("end"))
			if err1 != nil || err2 != nil {
				s.Bad++
				continue
			}
			style := strings.TrimSpace(get("style"))
			s.Styles[style]++
			text, align, drawing := parseText(get("text"))
			if drawing || text == "" {
				continue
			}
			raws = append(raws, raw{Event{Start: start, End: end, Style: style, Text: text, Order: len(raws)}, align})
		}
	}

	for _, r := range raws {
		align := r.align
		if align == 0 {
			align = lookupAlign(styleAlign, defaultAlign, r.ev.Style)
		}
		r.ev.Top = align >= 7
		s.Events = append(s.Events, r.ev)
	}
	return s
}

func columns(format string) map[string]int {
	m := map[string]int{}
	for i, c := range strings.Split(format, ",") {
		m[strings.ToLower(strings.TrimSpace(c))] = i
	}
	return m
}

// parseStyle returns the style name and its alignment as numpad (1-9).
func parseStyle(rest string, format map[string]int, legacy bool) (string, int, bool) {
	ni, ok := format["name"]
	if !ok {
		return "", 0, false
	}
	f := strings.Split(rest, ",")
	if ni >= len(f) {
		return "", 0, false
	}
	align := 2
	if ai, ok := format["alignment"]; ok && ai < len(f) {
		if v, err := strconv.Atoi(strings.TrimSpace(f[ai])); err == nil {
			if legacy {
				v = ssaToNumpad(v)
			}
			if v >= 1 && v <= 9 {
				align = v
			}
		}
	}
	return strings.TrimSpace(f[ni]), align, true
}

// lookupAlign resolves a style like libass: leading '*' ignored, "Default" in any
// case, and unknown names fall back to the Default style.
func lookupAlign(styles map[string]int, defaultAlign int, name string) int {
	name = strings.TrimLeft(name, "*")
	if strings.EqualFold(name, "Default") {
		name = "Default"
	}
	if a, ok := styles[name]; ok {
		return a
	}
	return defaultAlign
}

// ssaToNumpad converts SSA alignment (1-3 bottom, 5-7 top, 9-11 middle) to numpad.
// 4 and 8 count as 5, the VSFilter quirk libass copies.
func ssaToNumpad(v int) int {
	if v < 1 || v > 11 {
		return 0
	}
	if v&3 == 0 {
		v = 5
	}
	switch {
	case v&4 != 0:
		return v&3 + 6
	case v&8 != 0:
		return v&3 + 3
	}
	return v & 3
}

// parseText cleans a Text field. align is the first \an or \a override (0 if none
// or invalid: the first tag wins even when invalid, as in libass), drawing is true
// for \p1 or higher.
func parseText(raw string) (text string, align int, drawing bool) {
	seen := false
	for _, block := range overrideRe.FindAllString(raw, -1) {
		for _, m := range tagRe.FindAllStringSubmatch(block, -1) {
			v, _ := strconv.Atoi(m[2])
			switch m[1] {
			case "p":
				if v >= 1 {
					drawing = true
				}
			case "an":
				if !seen && v >= 1 && v <= 9 {
					align = v
				}
				seen = true
			case "a":
				if !seen {
					align = ssaToNumpad(v)
				}
				seen = true
			}
		}
	}
	s := escapes.Replace(overrideRe.ReplaceAllString(raw, ""))
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" { // an empty line would end the SRT cue
			lines = append(lines, l)
		}
	}
	return strings.Join(lines, "\n"), align, drawing
}

// parseTime reads H:MM:SS.cc into ms, without floats.
func parseTime(s string) (int, error) {
	hms, frac, _ := strings.Cut(strings.TrimSpace(s), ".")
	p := strings.Split(hms, ":")
	if len(p) != 3 {
		return 0, fmt.Errorf("tiempo inválido %q", s)
	}
	var n [3]int
	for i, x := range p {
		v, err := strconv.Atoi(x)
		if err != nil {
			return 0, fmt.Errorf("tiempo inválido %q", s)
		}
		n[i] = v
	}
	ms := 0
	if frac != "" {
		v, err := strconv.Atoi((frac + "00")[:3])
		if err != nil {
			return 0, fmt.Errorf("tiempo inválido %q", s)
		}
		ms = v
	}
	return ((n[0]*60+n[1])*60+n[2])*1000 + ms, nil
}

// WithoutStyles drops the events whose style matches re (nil keeps all).
func WithoutStyles(evs []Event, re *regexp.Regexp) []Event {
	if re == nil {
		return evs
	}
	return slices.DeleteFunc(slices.Clone(evs), func(e Event) bool { return re.MatchString(e.Style) })
}

// Merge sweeps every start and end time: each interval with active events becomes
// a block holding all of them, top-aligned first. Blocks never overlap.
func Merge(evs []Event, minGap int) []Block {
	var ts []int
	for _, e := range evs {
		if e.End > e.Start {
			ts = append(ts, e.Start, e.End)
		}
	}
	slices.Sort(ts)
	ts = slices.Compact(ts)

	var blocks []Block
	for i := 0; i+1 < len(ts); i++ {
		a, b := ts[i], ts[i+1]
		// ponytail: O(intervals × events), milliseconds for a few thousand lines; keep an active set if files grow far beyond that
		var act []Event
		for _, e := range evs {
			if e.Start <= a && a < e.End {
				act = append(act, e)
			}
		}
		if len(act) == 0 {
			continue
		}
		slices.SortFunc(act, func(x, y Event) int {
			if x.Top != y.Top {
				if x.Top {
					return -1
				}
				return 1
			}
			return cmp.Or(cmp.Compare(x.Start, y.Start), cmp.Compare(x.Order, y.Order))
		})
		var texts []string
		top := 0
		for _, e := range act {
			if !slices.Contains(texts, e.Text) {
				texts = append(texts, e.Text)
				if e.Top {
					top++
				}
			}
		}
		cur := Block{a, b, texts, top}
		if n := len(blocks); n > 0 && blocks[n-1].End == a && blocks[n-1].Top == top && blocks[n-1].Text() == cur.Text() {
			blocks[n-1].End = b
			continue
		}
		blocks = append(blocks, cur)
	}
	return dropMicro(blocks, minGap)
}

// dropMicro removes blocks shorter than minGap ms whose every text is shown by the
// nearest block that is not short, before or after. That block must touch it,
// directly or through a run of short blocks, so no text is ever lost.
func dropMicro(bs []Block, minGap int) []Block {
	short := func(b Block) bool { return b.End-b.Start < minGap }
	// solid walks from i in step d across touching short blocks and returns the
	// first block that is not short, or -1 if a gap or the end comes first.
	solid := func(i, d int) int {
		for j := i + d; j >= 0 && j < len(bs); j += d {
			if bs[min(j, j-d)].End != bs[max(j, j-d)].Start {
				return -1
			}
			if !short(bs[j]) {
				return j
			}
		}
		return -1
	}
	var out []Block
	for i, b := range bs {
		p, n := solid(i, -1), solid(i, 1)
		shown := func(t string) bool {
			return p >= 0 && slices.Contains(bs[p].Texts, t) || n >= 0 && slices.Contains(bs[n].Texts, t)
		}
		if short(b) && !slices.ContainsFunc(b.Texts, func(t string) bool { return !shown(t) }) {
			continue
		}
		out = append(out, b)
	}
	return out
}

// Warnings lists the blocks taller than 4 lines. They are never truncated.
func Warnings(bs []Block) []string {
	var w []string
	for _, b := range bs {
		if n := strings.Count(b.Text(), "\n") + 1; n > 4 {
			w = append(w, fmt.Sprintf(m.tallCue, srtTime(b.Start), n))
		}
	}
	return w
}

// padLine is a line with only a no-break space: the player keeps it as a blank
// line, while an empty line would end the cue.
const padLine = string(rune(0xA0))

// FormatSRT renders the blocks as SRT (UTF-8 without BOM, CRLF). A block with top
// texts gets blank lines between them and the dialogue until it is lines tall:
// a player that draws every cue at the bottom then shows the top texts near the
// top of the screen, at the same height whatever the dialogue. 0 pads nothing.
func FormatSRT(bs []Block, lines int) string {
	var sb strings.Builder
	for i, b := range bs {
		fmt.Fprintf(&sb, "%d\r\n%s --> %s\r\n%s\r\n\r\n", i+1, srtTime(b.Start), srtTime(b.End),
			strings.ReplaceAll(b.padded(lines), "\n", "\r\n"))
	}
	return sb.String()
}

func (b Block) padded(lines int) string {
	n := lines - strings.Count(b.Text(), "\n") - 1
	if b.Top == 0 || n <= 0 {
		return b.Text()
	}
	ls := append(slices.Clone(b.Texts[:b.Top]), slices.Repeat([]string{padLine}, n)...)
	return strings.Join(append(ls, b.Texts[b.Top:]...), "\n")
}

func srtTime(ms int) string {
	ms = max(ms, 0)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}
