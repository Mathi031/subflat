package main

// messages holds every string a user sees. The help is written by hand per
// language so it can have sections and examples; TestHelpListsEveryFlag keeps
// it in sync with the flags. m is the active language, chosen by setLang.
type messages struct {
	help string

	// usage errors
	errOneTarget, errJobs, errMinGap, errLines, errLang, usageHint string

	// before the batch
	noTools, notFound, notTarget, unknownFormat, dryRun, mkvFiles string

	// per file
	track, toSRT, sameAsSrc, noASS, exists, existsDry, events, styles, excluded,
	badLines, tallCue, emptyTrack, noDialogue, wouldWrite, extracting, remuxing,
	warning, skipped, failed, internal string

	// end of the batch
	summary, interrupted string
}

var m = en

var en = messages{
	help: `subflat %s - flatten ASS/SSA subtitles in MKV files into SRT any TV can show

Converts every ASS/SSA track of each MKV to SRT, merging the events that overlap
in time into one cue, and remuxes to a new MKV. The original is never modified.

USAGE
  subflat <folder | file.mkv> [options]

OUTPUT
  --out DIR              output folder (default: <folder>/tv)
  --recursive            also look in subfolders, mirroring the tree
  --keep-ass             keep the ASS tracks next to the new SRT ones
  --srt-only             write only the .srt files, no remux
  --force                overwrite outputs that already exist
  --jobs N               files in parallel (default 1; 2-3 only when writing to an SSD)

SUBTITLES
  --exclude-style REGEX  drop the events whose style matches, e.g. "(?i)sign|^op|^ed"
  --min-gap MS           drop cues shorter than MS whose text a neighbour already shows (default 100)
  --lines N              height, in lines, of the cues with top text, padded with blank lines
                         so signs show near the top; 0 keeps everything at the bottom (default 17)

GENERAL
  --dry-run              list tracks, styles and what would be written, write nothing
  --lang en|es           language of the messages (default: your system's, else en)
  --version              print the version
  -h, --help             this help

EXAMPLES
  subflat "D:\Anime\Season 1" --dry-run
  subflat "D:\Anime\Season 1" --recursive --exclude-style "(?i)^(signs?|op|ed)"
  subflat ~/Videos/Show --out /Volumes/USB/Show --keep-ass
  subflat episode.mkv --srt-only

Docs: https://github.com/Mathi031/subflat
`,
	errOneTarget: "give exactly one folder or .mkv file",
	errJobs:      "--jobs must be 1 or more",
	errMinGap:    "--min-gap cannot be negative",
	errLines:     "--lines cannot be negative",
	errLang:      "--lang must be en or es, not %q",
	usageHint:    "Run 'subflat --help' for usage.",

	noTools:       "mkvmerge and mkvextract (MKVToolNix) not found. Install it with:\n  %s\nor download it from https://mkvtoolnix.download/",
	notFound:      "%s does not exist",
	notTarget:     "%s is not a folder or an .mkv file",
	unknownFormat: "mkvmerge does not recognize the format",
	dryRun:        "dry run: nothing will be written",
	mkvFiles:      "MKV files",

	track:      "track",
	toSRT:      "-> SRT",
	sameAsSrc:  "the output would overwrite the original: use another --out",
	noASS:      "no ASS/SSA tracks",
	exists:     "%s already exists (use --force)",
	existsDry:  "%s already exists: without --force it would be skipped",
	events:     "track %d: %d events -> %d cues",
	styles:     "styles: %s",
	excluded:   "(excluded)",
	badLines:   "track %d: %d malformed Dialogue lines ignored",
	tallCue:    "%s: cue of %d lines",
	emptyTrack: "track %d has no dialogue after the filters: keeping the ASS",
	noDialogue: "no ASS track with dialogue",
	wouldWrite: "would write %s",
	extracting: "extracting subtitles",
	remuxing:   "remuxing %d%%",
	warning:    "warning:",
	skipped:    "skipped",
	failed:     "FAILED",
	internal:   "internal error: %v",

	summary:     "Done: converted %s, skipped %s, failed %s, warnings %s",
	interrupted: "Interrupted: some files were not processed.",
}

var es = messages{
	help: `subflat %s - aplana los subtítulos ASS/SSA de los MKV a SRT que cualquier TV muestra

Convierte cada pista ASS/SSA de cada MKV a SRT, fusionando en un solo subtítulo
los eventos que coinciden en el tiempo, y remuxea a un MKV nuevo. El original
nunca se modifica.

USO
  subflat <carpeta | archivo.mkv> [opciones]

SALIDA
  --out DIR              carpeta de salida (por defecto <carpeta>/tv)
  --recursive            busca también en subcarpetas y replica la estructura
  --keep-ass             conserva las pistas ASS junto a las SRT nuevas
  --srt-only             solo escribe los .srt, sin remux
  --force                sobrescribe las salidas que ya existen
  --jobs N               archivos en paralelo (1 por defecto; 2-3 solo si la salida va a un SSD)

SUBTÍTULOS
  --exclude-style REGEX  descarta los eventos cuyo estilo coincide, p. ej. "(?i)sign|^op|^ed"
  --min-gap MS           elimina los subtítulos de menos de MS cuyo texto ya muestra un vecino (100 por defecto)
  --lines N              alto, en líneas, de los subtítulos con texto de arriba, rellenados con líneas
                         en blanco para que el cartel salga arriba; 0 deja todo abajo (17 por defecto)

GENERAL
  --dry-run              muestra pistas, estilos y lo que se escribiría, sin escribir nada
  --lang en|es           idioma de los mensajes (por defecto el del sistema; si no, en)
  --version              muestra la versión
  -h, --help             esta ayuda

EJEMPLOS
  subflat "D:\Anime\Temporada 1" --dry-run
  subflat "D:\Anime\Temporada 1" --recursive --exclude-style "(?i)^(signs?|op|ed)"
  subflat ~/Videos/Serie --out /Volumes/USB/Serie --keep-ass
  subflat episodio.mkv --srt-only

Documentación: https://github.com/Mathi031/subflat/blob/main/README.es.md
`,
	errOneTarget: "indica una sola carpeta o un archivo .mkv",
	errJobs:      "--jobs tiene que ser 1 o más",
	errMinGap:    "--min-gap no puede ser negativo",
	errLines:     "--lines no puede ser negativo",
	errLang:      "--lang tiene que ser en o es, no %q",
	usageHint:    "Ejecuta 'subflat --help' para ver la ayuda.",

	noTools:       "no encuentro mkvmerge y mkvextract (MKVToolNix). Instálalo con:\n  %s\no descárgalo de https://mkvtoolnix.download/",
	notFound:      "%s no existe",
	notTarget:     "%s no es una carpeta ni un archivo .mkv",
	unknownFormat: "mkvmerge no reconoce el formato",
	dryRun:        "simulación: no se escribe nada",
	mkvFiles:      "Archivos MKV",

	track:      "pista",
	toSRT:      "-> SRT",
	sameAsSrc:  "la salida sobrescribiría el original: usa otro --out",
	noASS:      "sin pistas ASS/SSA",
	exists:     "%s ya existe (usa --force)",
	existsDry:  "%s ya existe: sin --force se saltaría",
	events:     "pista %d: %d eventos -> %d subtítulos",
	styles:     "estilos: %s",
	excluded:   "(excluido)",
	badLines:   "pista %d: %d líneas Dialogue mal formadas ignoradas",
	tallCue:    "%s: subtítulo de %d líneas",
	emptyTrack: "la pista %d queda sin diálogos tras los filtros: se conserva la ASS",
	noDialogue: "ninguna pista ASS con diálogos",
	wouldWrite: "se escribiría %s",
	extracting: "extrayendo subtítulos",
	remuxing:   "remuxeando %d%%",
	warning:    "aviso:",
	skipped:    "saltado",
	failed:     "ERROR",
	internal:   "error interno: %v",

	summary:     "Hecho: convertidos %s, saltados %s, con error %s, avisos %s",
	interrupted: "Interrumpido: algunos archivos quedaron sin procesar.",
}
