# subflat

[English](README.md) | Español

Aplana los subtítulos ASS/SSA de los MKV a SRT que cualquier TV muestra, incluidos
los reproductores de Samsung (Tizen) que dibujan un solo subtítulo a la vez.

## El problema

El reproductor del TV no renderiza ASS: ignora las posiciones y, cuando dos
eventos se solapan en el tiempo (dos personajes hablando a la vez, un cartel y un
diálogo), muestra solo uno. En VLC se ven todos, en el TV se pierden diálogos.

`subflat` convierte cada pista ASS/SSA a SRT y remuxea a un MKV nuevo. Los
eventos que coinciden en el tiempo se fusionan en un solo subtítulo, con las
líneas que iban arriba (carteles, un segundo personaje) antes que el diálogo.
Nunca hay dos subtítulos a la vez, así que el TV los muestra todos.

Como el TV dibuja cada subtítulo pegado abajo, entre el texto que iba arriba y el
diálogo se meten líneas en blanco hasta una altura fija (`--lines`): el cartel
queda cerca del borde superior y el diálogo abajo, como en VLC.

Vídeo, audios, capítulos, adjuntos y demás pistas se copian sin recodificar. Cada
pista ASS se reemplaza por su SRT en la misma posición, con el mismo idioma,
nombre y flags (default, forced y, si el original los marca, discapacidad
auditiva o visual, descripciones, idioma original y comentario). El archivo
original nunca se modifica.

## Instalación

Hace falta [MKVToolNix](https://mkvtoolnix.download/) (`mkvmerge` y `mkvextract`).
`subflat` lo busca en el PATH y, en Windows, también en `C:\Program Files\MKVToolNix`.

```powershell
winget install MoritzBunkus.MKVToolNix   # Windows
```

```sh
brew install mkvtoolnix                  # macOS
```

### Binarios

Descarga el de tu sistema desde la
[página de releases](https://github.com/Mathi031/subflat/releases):

| Archivo | Para |
|---|---|
| `subflat-windows-amd64.exe` | Windows 10/11 |
| `subflat-darwin-arm64` | Mac con Apple Silicon (M1 en adelante) |
| `subflat-darwin-amd64` | Mac con Intel |

Renómbralo a `subflat` (o `subflat.exe`) y ponlo en una carpeta del PATH. En
macOS, quita la cuarentena antes de usarlo:

```sh
chmod +x subflat
xattr -d com.apple.quarantine subflat
```

### Con Go

```sh
go install github.com/Mathi031/subflat@latest
```

Go se instala con `winget install GoLang.Go` o `brew install go`. El binario
queda en `%USERPROFILE%\go\bin` en Windows (el instalador de Go lo añade al PATH;
abre una terminal nueva) y en `~/go/bin` en macOS (si no está en el PATH, añade
`export PATH="$HOME/go/bin:$PATH"` a `~/.zshrc`).

Para compilar los tres binarios tú mismo, desde cualquier sistema con Go:
`go run build.go` los deja en `dist/`.

## Uso

```
subflat <carpeta | archivo.mkv> [opciones]
```

| Opción | Qué hace |
|---|---|
| `--out DIR` | Carpeta de salida. Por defecto `<carpeta>/tv`. |
| `--recursive` | Busca MKV también en subcarpetas y replica la estructura en la salida. La carpeta de salida nunca se vuelve a leer. |
| `--keep-ass` | Conserva las pistas ASS junto a las SRT nuevas. Cada ASS va justo detrás de su SRT, sin default ni forced, para que el TV elija la SRT. |
| `--exclude-style REGEX` | Descarta los eventos cuyo estilo coincide con la expresión (sintaxis RE2 de Go). `(?i)` ignora mayúsculas. |
| `--min-gap MS` | Umbral de los micro-intervalos, 100 por defecto. Ver más abajo. |
| `--lines N` | Alto, en líneas, de los subtítulos con texto de arriba, 17 por defecto. Se rellenan con líneas en blanco entre ese texto y el diálogo para que el cartel salga arriba. `0` deja todo abajo. Ver más abajo. |
| `--srt-only` | Solo escribe los `.srt` en la salida, como `<nombre>.<pista>.<idioma>.srt`, sin remux. |
| `--dry-run` | Muestra pistas, estilos con su conteo y lo que escribiría, sin escribir nada. |
| `--force` | Sobrescribe las salidas que ya existen. Sin él se saltan. |
| `--jobs N` | Archivos en paralelo, 1 por defecto. Usa 2 o 3 solo si la salida va a un SSD: en un disco mecánico o un USB, varias escrituras grandes a la vez se estorban. |
| `--lang en\|es` | Idioma de los mensajes. Ver más abajo. |
| `--version` | Muestra la versión. |

Las opciones aceptan uno o dos guiones (`-out` o `--out`) y pueden ir antes o
después del destino. `subflat` sin argumentos, o `subflat --help`, muestra la
ayuda integrada.

El destino puede ser una carpeta o un solo `.mkv`; con un archivo, la carpeta de
salida por defecto es `tv` junto a él.

### Idioma

Los mensajes y la ayuda están en inglés y en español. `subflat` sigue el idioma
del sistema (español en un Windows o macOS en español, inglés en cualquier otro).
Para forzarlo, pasa `--lang es` o `--lang en`, o define `SUBFLAT_LANG=es` en el
entorno. También respeta las variables de locale `LC_ALL`, `LC_MESSAGES` y `LANG`.

### Qué se ve

Con el MKV del test de integración, que tiene dos pistas ASS hechas con
`testdata/sample.ass` y una SRT:

```
subflat 1.0.0
D:\Anime\Serie  ->  D:\Anime\Serie\tv
Archivos MKV: 1

[1/1] [Grupo] Prueba ñandú 01.mkv
      pista 0  subtitles S_TEXT/ASS       es "Español" default  -> SRT
      pista 1  subtitles S_TEXT/UTF8      fr
      pista 2  subtitles S_TEXT/ASS       en "English" forced  -> SRT
      pista 0: 6 eventos -> 5 subtítulos
        estilos: Cartel 3, Default 3, Arriba 1
      pista 2: 6 eventos -> 5 subtítulos
        estilos: Cartel 3, Default 3, Arriba 1
  ok  [Grupo] Prueba ñandú 01.mkv  (1.2s)

Hecho: convertidos 1, saltados 0, con error 0, avisos 0  (1.2s)
```

El conteo de estilos sirve para decidir qué excluir: carteles, karaoke del
opening o del ending, notas en pantalla. Los subtítulos de más de 4 líneas de
texto salen como aviso con su tiempo; nunca se truncan.

En una terminal la salida va con color (`ok` verde, `saltado` amarillo, `ERROR`
rojo) y, con un archivo a la vez, el remux muestra su porcentaje en vivo. Con
`NO_COLOR=1` se quitan los colores; la salida redirigida o por tubería es texto
plano.

Los MKV sin pistas ASS se saltan. Un error en un archivo no detiene el lote.
Código de salida: 0 si todo fue bien, 1 si algún archivo falló o se interrumpió
la ejecución, 2 por un error de uso o por falta de MKVToolNix.

## Ejemplos

Ver qué hay antes de tocar nada, sobre todo los estilos:

```powershell
subflat "D:\Anime\Temporada 1" --dry-run
```

Quitar los carteles y el karaoke del opening y el ending, en toda la temporada:

```powershell
subflat "D:\Anime\Temporada 1" --recursive --exclude-style "(?i)^(signs?|op|ed)"
```

Escribir la salida en el USB del TV, conservando las ASS por si acaso:

```sh
subflat ~/Videos/Serie --out /Volumes/USB/Serie --keep-ass
```

Solo los SRT, para revisarlos:

```sh
subflat ~/Videos/Serie --srt-only --out ~/Videos/srt
```

Un solo episodio:

```sh
subflat episodio.mkv
```

## Cómo fusiona

1. Se ignoran los `Comment`, los eventos con dibujo vectorial (`\p1` o más) y los
   estilos excluidos.
2. Se limpia el texto: fuera los bloques `{...}`, `\N` y `\n` pasan a salto de
   línea, `\h` a espacio, y se descartan las líneas vacías.
3. Se toman todos los tiempos de inicio y fin. Cada tramo entre dos tiempos con
   algún evento activo es un subtítulo con todos esos eventos: primero los que
   iban arriba (alineación 7 a 9, por `\an`, por `\a` de SSA o por el estilo),
   luego por hora de inicio y por orden en el archivo. Los textos repetidos
   (capas de un mismo efecto) aparecen una sola vez.
4. Los tramos seguidos con el mismo texto y sin hueco se unen.
5. Un tramo de menos de `--min-gap` ms se elimina si todos sus textos ya se ven en
   el tramo largo más cercano, antes o después, pegado a él directamente o a
   través de otros tramos cortos. Así desaparece el parpadeo de 10 a 90 ms que
   deja una línea que termina justo después de que empiece la siguiente, sin
   perder texto.
6. Nunca se trunca. Un subtítulo de más de 4 líneas de texto sale como aviso,
   con su tiempo.
7. Al escribir la SRT, un subtítulo con texto de arriba se rellena con líneas en
   blanco (un espacio duro, U+00A0, por línea) entre ese texto y el diálogo hasta
   medir `--lines` líneas. El TV dibuja el bloque entero pegado abajo, así que el
   texto de arriba sube hasta cerca del borde superior y se queda a la misma
   altura aunque el diálogo tenga una línea, dos o ninguna. Los subtítulos sin
   texto de arriba no se tocan.

Un fragmento de `testdata/sample.ass`: Luis habla arriba (estilo `Arriba`,
alineación 8) mientras Ana sigue hablando abajo, y hay un cartel hecho con dos
capas del mismo texto:

```
Dialogue: 0,0:00:01.00,0:00:04.00,Default,Ana,0,0,0,,¿Dónde está el ñandú?
Dialogue: 0,0:00:02.00,0:00:05.00,Arriba,Luis,0,0,0,,{\i1}En el corral,{\i0} creo.
Dialogue: 1,0:00:02.00,0:00:05.00,Cartel,,0,0,0,,{\pos(960,300)\blur2}Granja Los Álamos
Dialogue: 0,0:00:02.00,0:00:05.00,Cartel,,0,0,0,,{\pos(960,300)\bord0}Granja Los Álamos
```

Sale una SRT sin solapes, con Luis primero mientras habla (con `--lines 0`, para
que se lea; con el valor por defecto, entre «En el corral, creo.» y el resto van
las líneas en blanco que completan las 17):

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

La SRT se escribe en UTF-8 sin BOM, con tiempos al milisegundo.

El valor por defecto de `--lines` está medido en un Samsung con el tamaño de
subtítulos «Pequeño»: 17 líneas dejan el cartel a un 10 % del borde superior,
con margen para que una línea larga que el TV parta en dos no lo saque de la
pantalla. Lo que no cabe por arriba el TV lo corta sin más: con 20 líneas el
cartel desaparece y solo queda el diálogo. Si en tu TV el cartel se corta, baja
el valor; si queda demasiado bajo, súbelo de uno en uno. Otros reproductores
usan otra letra: VLC, por ejemplo, con 17 líneas saca el diálogo por debajo de
la pantalla. Para ver esos archivos fuera del TV, genera con `--lines 0`.

Si una pista queda vacía tras los filtros, se conserva la ASS original y se avisa.
El remux se escribe primero como `<destino>.part` y se renombra al terminar, así
que una ejecución cortada no deja un archivo a medias que la siguiente daría por
hecho. Los temporales se borran siempre, también con error o Ctrl+C.

## Desarrollo

```sh
go vet ./...
go test ./...
```

El test de integración crea un MKV con `mkvmerge` y comprueba el resultado. Se
salta si MKVToolNix no está instalado. La CI corre lo mismo en cada push y pull
request; empujar un tag `v*` compila los binarios y publica un release.

## Limitaciones

- La SRT es texto plano: se pierden colores, fuentes, cursivas, cajas, efectos y
  posiciones exactas. Un cartel sube por el relleno de `--lines`, no por su
  posición real, y siempre centrado. Si los carteles molestan, se quitan con
  `--exclude-style`.
- No se usa `{\an8}` ni otra marca de posición: no son parte del estándar SRT y
  el reproductor de Samsung las muestra como texto, tapando además el diálogo
  que coincide con ellas.
- El relleno de `--lines` depende del tamaño de letra del reproductor. Está
  calibrado para el de Samsung con subtítulos «Pequeño»; con otro tamaño o en
  otro reproductor puede quedar más alto o más bajo.
- Solo convierte pistas ASS/SSA. Los subtítulos de imagen (PGS, VobSub) y las
  SRT que ya trae el MKV se copian tal cual.
- No recodifica: si el TV no soporta el vídeo o el audio del archivo, `subflat`
  no lo arregla.
- Comprobado con el reproductor nativo de un Samsung QN65Q65D (firmware
  T-NKLDUABC-0090-2123.9), desde USB: muestra todas las líneas del subtítulo
  fusionado, y con el relleno el cartel arriba y el diálogo abajo. Otros modelos
  y las apps de terceros (Plex, Jellyfin) pueden comportarse distinto.

## Contribuir

Los issues y pull requests son bienvenidos. Antes de abrir un PR, pasa
`go vet ./...` y `go test ./...` con MKVToolNix instalado para que corra también
el test de integración. Los commits siguen
[Conventional Commits](https://www.conventionalcommits.org/).

## Licencia

MIT
