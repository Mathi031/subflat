# subflat

English | [Español](README.es.md)

Flattens the ASS/SSA subtitles of MKV files into SRT that any TV can show,
including the Samsung (Tizen) players that draw one subtitle at a time.

## The problem

The TV's player does not render ASS: it ignores positions and, when two events
overlap in time (two characters talking at once, a sign over a dialogue), it
shows only one. VLC shows them all; the TV loses lines.

`subflat` converts every ASS/SSA track to SRT and remuxes into a new MKV. Events
that coincide in time are merged into one cue, with the lines that were at the
top (signs, a second speaker) before the dialogue. There are never two cues at
once, so the TV shows everything.

Since the TV draws every cue at the bottom, blank lines are inserted between the
top text and the dialogue up to a fixed height (`--lines`): the sign ends up near
the top edge and the dialogue at the bottom, like in VLC.

Video, audio, chapters, attachments and the other tracks are copied without
re-encoding. Each ASS track is replaced by its SRT in the same position, with the
same language, name and flags (default, forced and, when the original sets them,
hearing impaired, visual impaired, text descriptions, original language and
commentary). The original file is never modified.

## Install

You need [MKVToolNix](https://mkvtoolnix.download/) (`mkvmerge` and `mkvextract`).
`subflat` looks for it on the PATH and, on Windows, in `C:\Program Files\MKVToolNix`.

```powershell
winget install MoritzBunkus.MKVToolNix   # Windows
```

```sh
brew install mkvtoolnix                  # macOS
```

### Binaries

Download the one for your system from the
[releases page](https://github.com/Mathi031/subflat/releases):

| File | For |
|---|---|
| `subflat-windows-amd64.exe` | Windows 10/11 |
| `subflat-darwin-arm64` | Mac with Apple Silicon (M1 onwards) |
| `subflat-darwin-amd64` | Mac with Intel |

Rename it to `subflat` (or `subflat.exe`) and put it in a folder on your PATH. On
macOS, clear the quarantine before the first run:

```sh
chmod +x subflat
xattr -d com.apple.quarantine subflat
```

### With Go

```sh
go install github.com/Mathi031/subflat@latest
```

Go comes from `winget install GoLang.Go` or `brew install go`. The binary lands
in `%USERPROFILE%\go\bin` on Windows (the Go installer adds it to the PATH; open
a new terminal) and in `~/go/bin` on macOS (if that is not on your PATH, add
`export PATH="$HOME/go/bin:$PATH"` to `~/.zshrc`).

To build the three binaries yourself, from any system with Go: `go run build.go`
writes them to `dist/`.

## Usage

```
subflat <folder | file.mkv> [options]
```

| Option | What it does |
|---|---|
| `--out DIR` | Output folder. Default `<folder>/tv`. |
| `--recursive` | Also looks for MKV files in subfolders and mirrors the tree in the output. The output folder is never read back. |
| `--keep-ass` | Keeps the ASS tracks next to the new SRT ones. Each ASS goes right after its SRT, without default or forced, so the TV picks the SRT. |
| `--exclude-style REGEX` | Drops the events whose style matches the expression (Go RE2 syntax). `(?i)` ignores case. |
| `--min-gap MS` | Threshold for micro-cues, 100 by default. See below. |
| `--lines N` | Height, in lines, of the cues with top text, 17 by default. Blank lines are inserted between that text and the dialogue so the sign shows near the top. `0` keeps everything at the bottom. See below. |
| `--srt-only` | Only writes the `.srt` files to the output, as `<name>.<track>.<language>.srt`, no remux. |
| `--dry-run` | Lists tracks, styles with their counts and what would be written, without writing anything. |
| `--force` | Overwrites outputs that already exist. Without it they are skipped. |
| `--jobs N` | Files in parallel, 1 by default. Use 2 or 3 only when the output goes to an SSD: on a hard disk or a USB stick, several big writes at once slow each other down. |
| `--lang en\|es` | Language of the messages. See below. |
| `--version` | Prints the version. |

Options take one or two dashes (`-out` or `--out`) and can go before or after the
target. `subflat` with no arguments, or `subflat --help`, prints the built-in help.

The target can be a folder or a single `.mkv`; with a file, the default output
folder is `tv` next to it.

### Language

Messages and help come in English and Spanish. `subflat` follows your system
language (Spanish on a Spanish Windows or macOS, English anywhere else). To force
one, pass `--lang es` or `--lang en`, or set `SUBFLAT_LANG=es` in your
environment. The locale variables `LC_ALL`, `LC_MESSAGES` and `LANG` are honoured
too.

### What you see

With the MKV from the integration test, which has two ASS tracks made from
`testdata/sample.ass` and one SRT:

```
subflat 1.0.0
D:\Anime\Show  ->  D:\Anime\Show\tv
MKV files: 1

[1/1] [Grupo] Prueba ñandú 01.mkv
      track 0  subtitles S_TEXT/ASS       es "Español" default  -> SRT
      track 1  subtitles S_TEXT/UTF8      fr
      track 2  subtitles S_TEXT/ASS       en "English" forced  -> SRT
      track 0: 6 events -> 5 cues
        styles: Cartel 3, Default 3, Arriba 1
      track 2: 6 events -> 5 cues
        styles: Cartel 3, Default 3, Arriba 1
  ok  [Grupo] Prueba ñandú 01.mkv  (1.2s)

Done: converted 1, skipped 0, failed 0, warnings 0  (1.2s)
```

The style counts tell you what to exclude: signs, opening or ending karaoke,
on-screen notes. Cues taller than 4 lines of text are reported as warnings with
their time; they are never truncated.

On a terminal the output is colored (`ok` green, `skipped` yellow, `FAILED` red)
and, with one job at a time, the remux shows its percentage live. Set
`NO_COLOR=1` to turn colors off; piped or redirected output is plain text.

MKV files without ASS tracks are skipped. An error in one file does not stop the
batch. Exit code: 0 when everything went through, 1 if any file failed or the run
was interrupted, 2 for a usage error or missing MKVToolNix.

## Examples

See what is there before touching anything, above all the styles:

```powershell
subflat "D:\Anime\Season 1" --dry-run
```

Drop the signs and the opening and ending karaoke, for the whole season:

```powershell
subflat "D:\Anime\Season 1" --recursive --exclude-style "(?i)^(signs?|op|ed)"
```

Write the output to the TV's USB stick, keeping the ASS tracks just in case:

```sh
subflat ~/Videos/Show --out /Volumes/USB/Show --keep-ass
```

Only the SRT files, to review them:

```sh
subflat ~/Videos/Show --srt-only --out ~/Videos/srt
```

One episode:

```sh
subflat episode.mkv
```

## How it merges

1. `Comment` lines, events with vector drawings (`\p1` or higher) and excluded
   styles are ignored.
2. The text is cleaned: `{...}` blocks are removed, `\N` and `\n` become line
   breaks, `\h` a space, and empty lines are dropped.
3. Every start and end time is collected. Each span between two consecutive
   times with some active event is one cue holding all of them: first the ones
   that were at the top (alignment 7 to 9, by `\an`, by SSA `\a` or by the
   style), then by start time and file order. Repeated texts (layers of one
   effect) appear once.
4. Consecutive spans with the same text and no gap are joined.
5. A span shorter than `--min-gap` ms is dropped when every one of its texts is
   shown by the nearest long span, before or after, touching it directly or
   through other short spans. That removes the 10 to 90 ms flicker left by a
   line that ends just after the next one starts, without losing text.
6. Nothing is ever truncated. A cue with more than 4 lines of text is reported as
   a warning, with its time.
7. When writing the SRT, a cue with top text is padded with blank lines (one
   no-break space, U+00A0, per line) between that text and the dialogue until it
   is `--lines` tall. The TV draws the whole block at the bottom, so the top text
   rises to near the top edge and stays at the same height whether the dialogue
   has one line, two or none. Cues without top text are left alone.

A fragment of `testdata/sample.ass`: Luis speaks at the top (style `Arriba`,
alignment 8) while Ana keeps talking at the bottom, and there is a sign made of
two layers of the same text:

```
Dialogue: 0,0:00:01.00,0:00:04.00,Default,Ana,0,0,0,,¿Dónde está el ñandú?
Dialogue: 0,0:00:02.00,0:00:05.00,Arriba,Luis,0,0,0,,{\i1}En el corral,{\i0} creo.
Dialogue: 1,0:00:02.00,0:00:05.00,Cartel,,0,0,0,,{\pos(960,300)\blur2}Granja Los Álamos
Dialogue: 0,0:00:02.00,0:00:05.00,Cartel,,0,0,0,,{\pos(960,300)\bord0}Granja Los Álamos
```

Out comes an SRT without overlaps, with Luis first while he speaks (shown with
`--lines 0` so it stays readable; with the default, the blank lines that complete
the 17 go between "En el corral, creo." and the rest):

```
1
00:00:01,000 --> 00:00:02,000
¿Dónde está el ñandú?

2
00:00:02,000 --> 00:00:04,000
En el corral, creo.
¿Dónde está el ñandú?
Granja Los Álamos

3
00:00:04,000 --> 00:00:05,000
En el corral, creo.
Granja Los Álamos
```

The SRT is written in UTF-8 without BOM, with millisecond timestamps.

The default of `--lines` was measured on a Samsung with the subtitle size set to
"Small": 17 lines leave the sign about 10 % from the top edge, with room for a
long line the TV wraps in two. Whatever does not fit at the top the TV simply
cuts: with 20 lines the sign disappears and only the dialogue remains. If the
sign gets cut on your TV, lower the value; if it sits too low, raise it one at
a time. Other players use another font: VLC, for one, pushes the dialogue below
the screen with 17 lines. To watch those files outside the TV, generate them
with `--lines 0`.

If a track is empty after the filters, the original ASS is kept and a warning is
shown. The remux is written as `<destination>.part` and renamed at the end, so an
interrupted run never leaves a half file that the next run would take as done.
Temporary files are always removed, also on error or Ctrl+C.

## Development

```sh
go vet ./...
go test ./...
```

The integration test builds an MKV with `mkvmerge` and checks the result. It is
skipped when MKVToolNix is not installed. CI runs the same on every push and pull
request; pushing a `v*` tag builds the binaries and publishes a release.

## Limitations

- SRT is plain text: colors, fonts, italics, boxes, effects and exact positions
  are lost. A sign rises through the `--lines` padding, not through its real
  position, and always centered. If signs get in the way, drop them with
  `--exclude-style`.
- No `{\an8}` or other position tag is used: they are not part of the SRT
  standard and the Samsung player shows them as text, hiding the dialogue that
  coincides with them on top of that.
- The `--lines` padding depends on the player's font size. It is calibrated for
  the Samsung one with subtitles set to "Small"; with another size or in another
  player it may land higher or lower.
- Only ASS/SSA tracks are converted. Image subtitles (PGS, VobSub) and the SRT
  tracks the MKV already has are copied as they are.
- No re-encoding: if the TV does not support the file's video or audio, `subflat`
  does not fix that.
- Checked on the native player of a Samsung QN65Q65D (firmware
  T-NKLDUABC-0090-2123.9), from USB: it shows every line of the merged subtitle
  and, with the padding, the sign at the top and the dialogue at the bottom.
  Other models and third-party apps (Plex, Jellyfin) may behave differently.

## Contributing

Issues and pull requests are welcome. Before opening a PR, run `go vet ./...`
and `go test ./...` with MKVToolNix installed so the integration test runs too.
Commits follow [Conventional Commits](https://www.conventionalcommits.org/).

## License

MIT
