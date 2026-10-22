package main

import (
	"context"
	"strings"
	"testing"
)

func assTrack(id int, lang, ietf, name string) track {
	tr := track{ID: id, Type: "subtitles"}
	tr.Properties.CodecID = "S_TEXT/ASS"
	tr.Properties.Language, tr.Properties.LanguageIETF, tr.Properties.Name = lang, ietf, name
	return tr
}

func TestPick(t *testing.T) {
	spa := assTrack(2, "spa", "es-419", "Latino")
	spaSigns := assTrack(3, "spa", "es-419", "Carteles")
	eng := assTrack(4, "eng", "en", "")
	// ask answers with the track of that ID, or no answer when it is 0.
	ask := func(id int, asked *int) func([]track) (track, bool) {
		return func(cands []track) (track, bool) {
			*asked++
			for _, c := range cands {
				if c.ID == id {
					return c, true
				}
			}
			return track{}, false
		}
	}
	for _, tc := range []struct {
		name   string
		lang   string
		answer int // 0: no terminal
		ass    []track
		want   int // 0: skipped
		asks   int
	}{
		{"one track", "", 0, []track{eng}, 4, 0},
		{"several, no terminal", "", 0, []track{spa, eng}, 0, 0},
		{"several, asks", "", 4, []track{spa, eng}, 4, 1},
		{"no answer", "", 99, []track{spa, eng}, 0, 1},
		{"iso code", "spa", 0, []track{spa, eng}, 2, 0},
		{"ietf prefix", "ES", 0, []track{eng, spa}, 2, 0},
		{"exact ietf", "en", 0, []track{spa, eng}, 4, 0},
		{"no prefix inside a word", "e", 0, []track{spa, eng}, 0, 0},
		{"no match", "fre", 4, []track{spa, eng}, 0, 0},
		{"language leaves two, asks", "spa", 3, []track{spa, spaSigns, eng}, 3, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			asks := 0
			p := &picker{lang: tc.lang}
			if tc.answer != 0 {
				p.ask = ask(tc.answer, &asks)
			}
			got, again, err := p.pick(tc.ass)
			if tc.want == 0 && err == nil {
				t.Errorf("picked track %d, want skipped", got.ID)
			}
			if tc.want != 0 && (err != nil || got.ID != tc.want || again) {
				t.Errorf("got %d, again %v, err %v; want %d", got.ID, again, err, tc.want)
			}
			if asks != tc.asks {
				t.Errorf("asked %d times, want %d", asks, tc.asks)
			}
		})
	}
}

// A season asks once: the answer is reused while it names a single track by
// language and name, and asked again when it does not.
func TestPickRemembers(t *testing.T) {
	asks := 0
	p := &picker{ask: func(cands []track) (track, bool) { asks++; return cands[1], true }}
	ep1 := []track{assTrack(2, "spa", "es", "Latino"), assTrack(3, "eng", "en", "Full")}
	ep2 := []track{assTrack(3, "spa", "es", "Latino"), assTrack(4, "eng", "en", "Full")} // other IDs
	ep3 := []track{assTrack(2, "spa", "es", "Latino"), assTrack(3, "eng", "en", "Signs")}
	if tr, again, _ := p.pick(ep1); tr.ID != 3 || again || asks != 1 {
		t.Fatalf("ep1: %d, again %v, asks %d", tr.ID, again, asks)
	}
	if tr, again, _ := p.pick(ep2); tr.ID != 4 || !again || asks != 1 {
		t.Errorf("ep2: %d, again %v, asks %d; want 4 without asking", tr.ID, again, asks)
	}
	if tr, again, _ := p.pick(ep3); tr.ID != 3 || again || asks != 2 {
		t.Errorf("ep3: %d, again %v, asks %d; want a new question", tr.ID, again, asks)
	}
}

// askTrack keeps asking until it reads a valid number; the end of the input and
// Ctrl+C are no answer.
func TestAskTrack(t *testing.T) {
	ctx := context.Background()
	cands := []track{assTrack(2, "spa", "es", ""), assTrack(5, "eng", "en", "")}
	ask := func(in string) (track, bool) { return askTrack(ctx, readLines(strings.NewReader(in)), cands) }
	if tr, ok := ask("x\n0\n3\r\n 2 \r\n"); !ok || tr.ID != 5 {
		t.Errorf("got %d, %v; want track 5", tr.ID, ok)
	}
	if _, ok := ask("x\n"); ok {
		t.Error("end of input should be no answer")
	}
	if tr, ok := ask("1"); !ok || tr.ID != 2 {
		t.Errorf("last line without newline: got %d, %v; want track 2", tr.ID, ok)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, ok := askTrack(canceled, make(chan string), cands); ok {
		t.Error("Ctrl+C should be no answer")
	}
}

// --burn-lang narrows the candidates first; the remembered answer then picks
// among them.
func TestPickLangAndMemory(t *testing.T) {
	asks := 0
	p := &picker{lang: "spa", ask: func(cands []track) (track, bool) { asks++; return cands[1], true }}
	ep := []track{assTrack(2, "spa", "es", "Latino"), assTrack(3, "spa", "es", "Carteles"), assTrack(4, "eng", "en", "")}
	for i := range 2 {
		if tr, again, err := p.pick(ep); err != nil || tr.ID != 3 || again != (i == 1) || asks != 1 {
			t.Errorf("file %d: %d, again %v, err %v, asks %d", i+1, tr.ID, again, err, asks)
		}
	}
}

// The statistics tags win; without them the container duration gives an
// estimate; without either, no percentage.
func TestFrameCount(t *testing.T) {
	var info mkvInfo
	info.Container.Properties.Duration = 10e9
	var v track
	if n := frameCount(info, v); n != 0 {
		t.Errorf("nothing known: %d", n)
	}
	v.Properties.FrameDur = 41708333
	if n := frameCount(info, v); n != 239 {
		t.Errorf("from the duration: %d, want 239", n)
	}
	v.Properties.Frames = "72"
	if n := frameCount(info, v); n != 72 {
		t.Errorf("from the tags: %d, want 72", n)
	}
}
