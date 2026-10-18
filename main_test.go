package main

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	c, err := parseArgs([]string{"--jobs", "2", "series", "--keep-ass", "--exclude-style", "(?i)sign", "--min-gap=50"})
	if err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs("series")
	if c.dir != dir || c.file != "" || c.out != filepath.Join(dir, "tv") || c.jobs != 2 || !c.keepASS ||
		c.minGap != 50 || c.exclude.String() != "(?i)sign" {
		t.Errorf("config = %+v", c)
	}
	for _, bad := range [][]string{{"a", "b"}, {"a", "--jobs", "0"}, {"a", "--exclude-style", "("}, {"a", "--lang", "fr"}, {"a", "--nope"}} {
		if _, err := parseArgs(bad); err == nil {
			t.Errorf("parseArgs(%q) should fail", bad)
		}
	}
	if _, err := parseArgs(nil); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("no args: err = %v, want ErrHelp", err)
	}
	if _, err := parseArgs([]string{"--version"}); !errors.Is(err, errVersion) {
		t.Errorf("--version: err = %v, want errVersion", err)
	}
}

func TestParseArgsSingleFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "ep.mkv")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := parseArgs([]string{file})
	if err != nil {
		t.Fatal(err)
	}
	if c.file != file || c.dir != dir || c.out != filepath.Join(dir, "tv") {
		t.Errorf("config = %+v", c)
	}
	if files, err := findMKVs(c); err != nil || len(files) != 1 || files[0] != file {
		t.Errorf("findMKVs = %q, %v", files, err)
	}
}

func TestSetLang(t *testing.T) {
	t.Cleanup(func() { m = en })
	t.Setenv("SUBFLAT_LANG", "es")
	if err := setLang(""); err != nil || m != es {
		t.Errorf("env es: err = %v, m is es = %v", err, m == es)
	}
	if err := setLang("en"); err != nil || m != en {
		t.Errorf("flag beats env: err = %v, m is en = %v", err, m == en)
	}
	if err := setLang("fr"); err == nil {
		t.Error("fr should fail")
	}
	t.Setenv("SUBFLAT_LANG", "")
	t.Setenv("LC_ALL", "es_ES.UTF-8")
	if err := setLang(""); err != nil || m != es {
		t.Errorf("LC_ALL es: err = %v, m is es = %v", err, m == es)
	}
	t.Setenv("LC_ALL", "C")
	if err := setLang(""); err != nil || m != en {
		t.Errorf("LC_ALL C: err = %v, m is en = %v", err, m == en)
	}
	if got := langArg([]string{"x", "--lang=es"}); got != "es" {
		t.Errorf("langArg = %q", got)
	}
	if got := langArg([]string{"-lang", "en", "x"}); got != "en" {
		t.Errorf("langArg = %q", got)
	}
}

// The help is written by hand: every flag has to appear in both languages.
func TestHelpListsEveryFlag(t *testing.T) {
	var c config
	var s1, s2 string
	var b bool
	newFlags(&c, &s1, &s2, &b).VisitAll(func(f *flag.Flag) {
		for lang, h := range map[string]string{"en": en.help, "es": es.help} {
			if !strings.Contains(h, "  --"+f.Name+" ") {
				t.Errorf("%s help does not list --%s", lang, f.Name)
			}
		}
	})
	for _, h := range []string{en.help, es.help} {
		if strings.Count(h, "%") != 1 {
			t.Errorf("help must have exactly one %%s, for the version")
		}
	}
}

func TestBoldHeadings(t *testing.T) {
	color = true
	t.Cleanup(func() { color = false })
	got := boldHeadings("subflat 1 - x\n\nUSAGE\n  --out DIR\nDocs: y\n")
	if want := "subflat 1 - x\n\n\x1b[1mUSAGE\x1b[0m\n  --out DIR\nDocs: y\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
