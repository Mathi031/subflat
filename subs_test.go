package main

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

const header = `[Script Info]
ScriptType: v4.00+

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1
Style: Top,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,8,10,10,10,1
Style: Signs,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,5,10,10,10,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
`

func script(lines ...string) string { return header + strings.Join(lines, "\n") + "\n" }

func dlg(start, end, style, text string) string {
	return "Dialogue: 0," + start + "," + end + "," + style + ",,0,0,0,," + text
}

// dump renders blocks as "start-end text|text" lines for readable diffs.
func dump(bs []Block) string {
	var sb strings.Builder
	for _, b := range bs {
		sb.WriteString(srtTime(b.Start) + "-" + srtTime(b.End) + " " + strings.Join(b.Texts, "|") + "\n")
	}
	return sb.String()
}

func merged(src string, minGap int) string { return dump(Merge(ParseASS(src).Events, minGap)) }

func TestMerge(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		minGap int
		want   string
	}{
		{"total overlap", script(
			dlg("0:00:00.00", "0:00:05.00", "Default", "A"),
			dlg("0:00:01.00", "0:00:02.00", "Default", "B"),
		), 100, `00:00:00,000-00:00:01,000 A
00:00:01,000-00:00:02,000 A|B
00:00:02,000-00:00:05,000 A
`},
		{"partial overlap", script(
			dlg("0:00:00.00", "0:00:03.00", "Default", "A"),
			dlg("0:00:02.00", "0:00:05.00", "Default", "B"),
		), 100, `00:00:00,000-00:00:02,000 A
00:00:02,000-00:00:03,000 A|B
00:00:03,000-00:00:05,000 B
`},
		{"same times keep file order", script(
			dlg("0:00:01.00", "0:00:02.00", "Default", "B"),
			dlg("0:00:01.00", "0:00:02.00", "Default", "A"),
		), 100, "00:00:01,000-00:00:02,000 B|A\n"},
		{"earlier start goes first", script(
			dlg("0:00:01.00", "0:00:03.00", "Default", "late"),
			dlg("0:00:00.00", "0:00:03.00", "Default", "early"),
		), 100, `00:00:00,000-00:00:01,000 early
00:00:01,000-00:00:03,000 early|late
`},
		{"top by \\an8 goes first", script(
			dlg("0:00:00.00", "0:00:02.00", "Default", "abajo"),
			dlg("0:00:01.00", "0:00:02.00", "Default", `{\an8}arriba`),
		), 100, `00:00:00,000-00:00:01,000 abajo
00:00:01,000-00:00:02,000 arriba|abajo
`},
		{"top by SSA \\a6 and by style", script(
			dlg("0:00:00.00", "0:00:01.00", "Default", "abajo"),
			dlg("0:00:00.00", "0:00:01.00", "Top", "estilo"),
			dlg("0:00:00.00", "0:00:01.00", "Default", `{\a6}ssa`),
		), 100, "00:00:00,000-00:00:01,000 estilo|ssa|abajo\n"},
		{"top lines first, then by start", script(
			dlg("0:00:00.00", "0:00:03.00", "Top", "A"),
			dlg("0:00:01.00", "0:00:04.00", "Default", "B"),
			dlg("0:00:02.00", "0:00:03.00", "Top", "C"),
		), 100, `00:00:00,000-00:00:01,000 A
00:00:01,000-00:00:02,000 A|B
00:00:02,000-00:00:03,000 A|C|B
00:00:03,000-00:00:04,000 B
`},
		{"\\an2 overrides a top style", script(
			dlg("0:00:00.00", "0:00:01.00", "Default", "primero"),
			dlg("0:00:00.00", "0:00:01.00", "Top", `{\an2}abajo`),
		), 100, "00:00:00,000-00:00:01,000 primero|abajo\n"},
		{"tags, line breaks and hard spaces", script(
			dlg("0:00:00.00", "0:00:01.00", "Default", `{\i1}Hola,{\i0} mundo\Nsegunda\hlínea\n {\b1} `),
		), 100, "00:00:00,000-00:00:01,000 Hola, mundo\nsegunda línea\n"},
		{"empty text after cleaning is dropped", script(
			dlg("0:00:00.00", "0:00:01.00", "Default", `{\fad(200,200)}\N  \h`),
		), 100, ""},
		{"drawings dropped, \\pos kept", script(
			dlg("0:00:00.00", "0:00:01.00", "Default", `{\p1}m 0 0 l 100 0 100 100{\p0}`),
			dlg("0:00:00.00", "0:00:01.00", "Default", `{\pos(10,10)\pbo0}cartel`),
		), 100, "00:00:00,000-00:00:01,000 cartel\n"},
		{"comments ignored", script(
			"Comment: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,nota",
			dlg("0:00:00.00", "0:00:01.00", "Default", "texto"),
		), 100, "00:00:00,000-00:00:01,000 texto\n"},
		{"identical texts deduplicated", script(
			dlg("0:00:00.00", "0:00:01.00", "Default", `{\blur3}Hola`),
			dlg("0:00:00.00", "0:00:01.00", "Default", `{\bord0}Hola`),
		), 100, "00:00:00,000-00:00:01,000 Hola\n"},
		{"identical consecutive blocks join", script(
			dlg("0:00:00.00", "0:00:01.00", "Default", "A"),
			dlg("0:00:01.00", "0:00:02.00", "Default", "A"),
			dlg("0:00:02.50", "0:00:03.00", "Default", "A"),
		), 100, `00:00:00,000-00:00:02,000 A
00:00:02,500-00:00:03,000 A
`},
		{"micro block dropped when neighbors show its text", script(
			dlg("0:00:00.00", "0:00:01.00", "Default", "A"),
			dlg("0:00:00.96", "0:00:03.00", "Default", "B"),
		), 100, `00:00:00,000-00:00:00,960 A
00:00:01,000-00:00:03,000 B
`},
		{"micro block kept below min-gap", script(
			dlg("0:00:00.00", "0:00:01.00", "Default", "A"),
			dlg("0:00:00.96", "0:00:03.00", "Default", "B"),
		), 40, `00:00:00,000-00:00:00,960 A
00:00:00,960-00:00:01,000 A|B
00:00:01,000-00:00:03,000 B
`},
		{"micro block kept without a touching neighbor", script(
			dlg("0:00:00.00", "0:00:01.00", "Default", "A"),
			dlg("0:00:01.50", "0:00:01.55", "Default", "A"),
		), 100, `00:00:00,000-00:00:01,000 A
00:00:01,500-00:00:01,550 A
`},
		{"run of micro blocks dropped when the blocks around show their texts", script(
			dlg("0:00:00.00", "0:00:03.00", "Default", "A"),
			dlg("0:00:01.00", "0:00:02.90", "Default", "B"),
			dlg("0:00:02.99", "0:00:05.00", "Default", "C"),
		), 100, `00:00:00,000-00:00:01,000 A
00:00:01,000-00:00:02,900 A|B
00:00:03,000-00:00:05,000 C
`},
		{"micro block kept when its neighbor is micro too", script(
			dlg("0:00:00.00", "0:00:01.04", "Default", "A"),
			dlg("0:00:01.00", "0:00:01.08", "Default", "B"),
		), 100, `00:00:00,000-00:00:01,000 A
00:00:01,000-00:00:01,040 A|B
00:00:01,040-00:00:01,080 B
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := merged(tt.src, tt.minGap); got != tt.want {
				t.Errorf("got\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestFormatColumnsAndCommas(t *testing.T) {
	src := `[V4+ Styles]
Format: Alignment, Name
Style: 8,Arriba

[Events]
Format: Style, End, Start, Layer, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: Default,0:00:02.00,0:00:01.00,0,,0,0,0,,Uno, dos, tres
Dialogue: *Arriba,0:00:02.00,0:00:01.00,0,,0,0,0,,cartel
`
	got := merged(src, 100)
	want := "00:00:01,000-00:00:02,000 cartel|Uno, dos, tres\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBOMAndCRLF(t *testing.T) {
	src := "\xef\xbb\xbf[V4+ Styles]\r\nFormat: Name, Alignment\r\nStyle: Default,8\r\n\r\n[Events]\r\n" +
		"Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\r\n" +
		"Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,hola\r\n"
	s := ParseASS(src)
	if len(s.Events) != 1 || s.Events[0].Text != "hola" || !s.Events[0].Top {
		t.Errorf("events = %+v", s.Events)
	}
}

func TestSSAStyles(t *testing.T) {
	src := `[V4 Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, TertiaryColour, BackColour, Bold, Italic, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, AlphaLevel, Encoding
Style: Default,Arial,20,16777215,65535,65535,0,0,0,1,2,2,2,10,10,10,0,1
Style: Arriba,Arial,20,16777215,65535,65535,0,0,0,1,2,2,6,10,10,10,0,1
Style: Medio,Arial,20,16777215,65535,65535,0,0,0,1,2,2,10,10,10,10,0,1

[Events]
Format: Marked, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: Marked=0,0:00:00.00,0:00:01.00,Medio,,0,0,0,,medio
Dialogue: Marked=0,0:00:00.00,0:00:01.00,Default,,0,0,0,,abajo
Dialogue: Marked=0,0:00:00.00,0:00:01.00,Arriba,,0,0,0,,arriba
Dialogue: Marked=0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\a9}medio a9
`
	got := merged(src, 100)
	want := "00:00:00,000-00:00:01,000 arriba|medio|abajo|medio a9\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSSAToNumpad(t *testing.T) {
	for v, want := range map[int]int{1: 1, 2: 2, 3: 3, 4: 7, 5: 7, 6: 8, 7: 9, 8: 7, 9: 4, 10: 5, 11: 6, 0: 0, 12: 0} {
		if got := ssaToNumpad(v); got != want {
			t.Errorf("ssaToNumpad(%d) = %d, want %d", v, got, want)
		}
	}
}

func TestFirstAlignmentTagWins(t *testing.T) {
	for raw, want := range map[string]int{
		`{\an8}{\an2}x`:  8,
		`{\a6\an2}x`:     8,
		`{\an0\an8}x`:    0, // invalid first tag still counts: the style decides
		`{\alpha&HFF&}x`: 0,
	} {
		if _, got, _ := parseText(raw); got != want {
			t.Errorf("parseText(%q) align = %d, want %d", raw, got, want)
		}
	}
}

func TestStylesAndExclusion(t *testing.T) {
	s := ParseASS(script(
		dlg("0:00:00.00", "0:00:01.00", "Default", "uno"),
		dlg("0:00:01.00", "0:00:02.00", "Default", "dos"),
		dlg("0:00:00.00", "0:00:02.00", "Signs", "cartel"),
		dlg("0:00:00.00", "0:00:02.00", "Signs", `{\p1}m 0 0`),
		"Comment: 0,0:00:00.00,0:00:01.00,OP,,0,0,0,,karaoke",
		"Dialogue: roto",
	))
	if s.Styles["Default"] != 2 || s.Styles["Signs"] != 2 || len(s.Styles) != 2 {
		t.Errorf("styles = %v", s.Styles)
	}
	if s.Bad != 1 {
		t.Errorf("bad = %d, want 1", s.Bad)
	}
	got := dump(Merge(WithoutStyles(s.Events, regexp.MustCompile(`(?i)^signs$`)), 100))
	want := "00:00:00,000-00:00:01,000 uno\n00:00:01,000-00:00:02,000 dos\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWarnings(t *testing.T) {
	bs := []Block{
		{0, 1000, []string{"1\n2", "3\n4"}, 0},
		{1000, 2000, []string{"1\n2\n3", "4\n5"}, 0},
	}
	w := Warnings(bs)
	if len(w) != 1 || w[0] != "00:00:01,000: cue of 5 lines" {
		t.Errorf("warnings = %q", w)
	}
}

func TestMergeTop(t *testing.T) {
	bs := Merge(ParseASS(script(
		dlg("0:00:00.00", "0:00:02.00", "Default", "abajo"),
		dlg("0:00:01.00", "0:00:03.00", "Default", `{\an8}arriba`),
		dlg("0:00:01.00", "0:00:03.00", "Top", "otro"),
		dlg("0:00:01.00", "0:00:03.00", "Default", "arriba"), // same text as a top one: counted once, as top
	)).Events, 0)
	var tops []int
	for _, b := range bs {
		tops = append(tops, b.Top)
	}
	if want := []int{0, 2, 2}; !slices.Equal(tops, want) {
		t.Errorf("tops = %v, want %v\n%s", tops, want, dump(bs))
	}
}

func TestFormatSRT(t *testing.T) {
	got := FormatSRT([]Block{
		{3723004, 3724500, []string{"Préstamo Mágico\nHabilidad", "hay una mazmorra"}, 1},
		{36000000, 36000001, []string{"diez horas"}, 0},
	}, 0)
	want := "1\r\n01:02:03,004 --> 01:02:04,500\r\nPréstamo Mágico\r\nHabilidad\r\nhay una mazmorra\r\n\r\n" +
		"2\r\n10:00:00,000 --> 10:00:00,001\r\ndiez horas\r\n\r\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatSRTLines(t *testing.T) {
	// Bytes, not a string literal: the same typo in the constant and in the
	// expectations would cancel out.
	if b := []byte(padLine); len(b) != 2 || b[0] != 0xC2 || b[1] != 0xA0 {
		t.Fatalf("padLine = % x, want c2 a0 (U+00A0)", b)
	}
	nb := padLine + "\r\n"
	cases := []struct {
		name  string
		b     Block
		lines int
		want  string // cue text, CRLF after each line
	}{
		{"sign over dialogue", Block{0, 1000, []string{"cartel\ndos", "habla"}, 1}, 6, "cartel\r\ndos\r\n" + nb + nb + nb + "habla\r\n"},
		{"sign alone pads after it", Block{0, 1000, []string{"cartel"}, 1}, 3, "cartel\r\n" + nb + nb},
		{"two signs, two dialogues", Block{0, 1000, []string{"a", "b", "c", "d"}, 2}, 5, "a\r\nb\r\n" + nb + "c\r\nd\r\n"},
		{"already tall enough", Block{0, 1000, []string{"a", "b"}, 1}, 2, "a\r\nb\r\n"},
		{"no sign, no padding", Block{0, 1000, []string{"a"}, 0}, 6, "a\r\n"},
		{"lines 0 disables", Block{0, 1000, []string{"a", "b"}, 1}, 0, "a\r\nb\r\n"},
	}
	for _, c := range cases {
		want := "1\r\n00:00:00,000 --> 00:00:01,000\r\n" + c.want + "\r\n"
		if got := FormatSRT([]Block{c.b}, c.lines); got != want {
			t.Errorf("%s: got %q, want %q", c.name, got, want)
		}
	}
}

func TestParseTime(t *testing.T) {
	for in, want := range map[string]int{"0:00:01.50": 1500, "1:02:03.04": 3723040, "0:00:00.5": 500, "0:00:00.123": 123, " 0:00:02.00 ": 2000} {
		if got, err := parseTime(in); err != nil || got != want {
			t.Errorf("parseTime(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	if _, err := parseTime("1:00"); err == nil {
		t.Error("parseTime(1:00) should fail")
	}
}
