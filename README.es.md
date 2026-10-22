# subflat

[English](README.md) | Español

Aplana los subtítulos ASS/SSA de los MKV a SRT que cualquier TV muestra, incluidos
los reproductores de Samsung (Tizen) que dibujan un solo subtítulo a la vez. O,
con `--burn`, los dibuja sobre el vídeo tal como los muestra VLC.

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

Para `--burn` hace falta además [ffmpeg](https://ffmpeg.org/) compilado con
libass, en el PATH. Estas versiones lo traen:

```powershell
winget install Gyan.FFmpeg               # Windows (o bien: choco install ffmpeg)
```

```sh
brew install ffmpeg                      # macOS
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
| `--burn` | Dibuja una pista ASS sobre el vídeo en vez de convertir a SRT. Ver [Quemar los subtítulos](#quemar-los-subtítulos). No se combina con `--srt-only`, `--keep-ass`, `--exclude-style`, `--min-gap` ni `--lines`. |
| `--burn-lang IDIOMA` | Con `--burn`, la pista a quemar cuando hay varias: `spa`, `es` (acepta también `es-419`), `eng`... Sin él, `subflat` pregunta. |
| `--encoder NOMBRE` | Con `--burn`: `auto` (por defecto), `qsv` (Intel Quick Sync) o `x265` (CPU). |
| `--quality N` | Con `--burn`, de 1 a 51, 18 por defecto. Menor es mejor y más pesado. |
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
la ejecución, 2 por un error de uso o por falta de una herramienta (MKVToolNix,
o ffmpeg con `--burn`).

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

Quemar la pista en español en toda la temporada, directo al USB del TV:

```powershell
subflat "D:\Anime\Temporada 1" --burn --burn-lang spa --out "E:\Temporada 1"
```

## Cómo fusiona

1. Se ignoran los `Comment`, los eventos con dibujo vectorial (`\p1` o más) y los
   estilos excluidos. También el karaoke hecho con un generador (el templater de
   Aegisub lo marca con el efecto `fx`, Kara Effector lo firma): llega como un
   evento por letra o sílaba, y cada uno sería una línea. En su lugar van los
   `Comment` de esos mismos estilos, las líneas originales que el generador
   conserva; si el archivo no las conservó, esa canción queda sin letra. El
   informe dice cuántas líneas se descartaron y cuántas se recuperaron.
2. Se limpia el texto: fuera los bloques `{...}`, `\N` y `\n` pasan a salto de
   línea, `\h` a espacio, y se descartan las líneas vacías.
3. Se toman todos los tiempos de inicio y fin. Cada tramo entre dos tiempos con
   algún evento activo es un subtítulo con todos esos eventos: primero los que
   iban arriba (alineación 7 a 9, por `\an`, por `\a` de SSA o por el estilo; un
   evento colocado con `\pos` o `\move` va arriba si ese punto está en la mitad
   superior de la imagen, según `PlayResY`), luego por hora de inicio y por orden
   en el archivo. Los textos repetidos
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
capas del mismo texto, colocado en la mitad superior de una imagen de 1080 líneas:

```
Dialogue: 0,0:00:01.00,0:00:04.00,Default,Ana,0,0,0,,¿Dónde está el ñandú?
Dialogue: 0,0:00:02.00,0:00:05.00,Arriba,Luis,0,0,0,,{\i1}En el corral,{\i0} creo.
Dialogue: 1,0:00:02.00,0:00:05.00,Cartel,,0,0,0,,{\pos(960,300)\blur2}Granja Los Álamos
Dialogue: 0,0:00:02.00,0:00:05.00,Cartel,,0,0,0,,{\pos(960,300)\bord0}Granja Los Álamos
```

Sale una SRT sin solapes, con Luis y el cartel primero mientras están en pantalla
(con `--lines 0`, para que se lea; con el valor por defecto, entre «Granja Los
Álamos» y el resto van las líneas en blanco que completan las 17):

```
1
00:00:01,000 --> 00:00:02,000
¿Dónde está el ñandú?

2
00:00:02,000 --> 00:00:04,000
En el corral, creo.
Granja Los Álamos
¿Dónde está el ñandú?

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

## Quemar los subtítulos

El texto SRT no admite posiciones, cuadros ni animación, y el reproductor de
Samsung ignora los subtítulos de imagen (PGS). La única forma de ver en el TV lo
que muestra VLC (carteles en su sitio, cuadros, karaoke que se mueve) es
dibujarlos sobre los fotogramas. Eso hace `--burn`:

1. Elige una pista ASS. Si hay una sola, esa. Si hay varias, `--burn-lang` elige
   por idioma; sin él, `subflat` las lista y pide un número. La respuesta se
   reutiliza en los siguientes archivos del lote mientras coincida con una sola
   pista por idioma y nombre, así que una temporada pregunta una vez. Sin una
   terminal en la que preguntar, un archivo con varias pistas se salta.
2. Extrae esa pista y los adjuntos (las fuentes) a una carpeta temporal, y ffmpeg
   la dibuja con libass, el mismo motor que usa VLC, sobre cada fotograma.
3. El vídeo se recodifica a HEVC 10 bit; los fps, el número de fotogramas y las
   marcas de tiempo quedan como en el original. El audio se copia sin recodificar.
4. `mkvmerge` junta el vídeo nuevo con el audio, los capítulos y las etiquetas del
   original. La salida no lleva pistas de subtítulos (el TV podría activar una
   encima de la imagen) ni adjuntos.

El codificador: `auto` usa Intel Quick Sync (`qsv`) si codifica un fotograma de
prueba y, si no, x265 en la CPU (y falla si ninguno funciona). Dos episodios de
24 minutos en 1080p, quemados con `qsv` y la calidad por defecto en un i7-1260P
con su gráfica integrada. VMAF compara escenas de 30 s contra el original con los
subtítulos dibujados y sin comprimir (100 es idéntico; por encima de 95 la
diferencia apenas se ve):

| Original | Quemado | Tiempo | VMAF |
|---|---|---|---|
| HEVC 10 bit, vídeo 6,4 Mbit/s, opening con karaoke, 1,20 GB | vídeo 2,6 Mbit/s, 499 MB | 6 min 55 s | 96,7 a 97,8 |
| H.264, vídeo 3,1 Mbit/s, 196 fuentes adjuntas, 1,09 GB | vídeo 2,0 Mbit/s, 405 MB | 7 min 34 s | 96,1 a 97,3 |

Los dos conservaron todos los fotogramas (34.167 y 34.047) y todo el audio. x265
dio el mismo tamaño y la misma calidad (VMAF 98,4 frente a 97,8 en la escena de
karaoke) unas 13 veces más lento: cerca de 2 horas por episodio en esa CPU.

La calidad fija un aspecto, no un tamaño: las escenas tranquilas gastan pocos
bits y las movidas pueden gastar más que el original (10,2 Mbit/s frente a 3,5 en
una escena con mucho movimiento), así que el tamaño depende del episodio. La
salida descarta los adjuntos, que en algunos lanzamientos son cientos de MB de
fuentes. El vídeo codificado pasa por la carpeta temporal antes del remux final,
que necesita allí un espacio libre parecido al tamaño de la salida.

## Desarrollo

```sh
go vet ./...
go test ./...
```

Los tests de integración crean un MKV con `mkvmerge` y comprueban el resultado.
Se saltan si MKVToolNix no está instalado, y el de `--burn` también sin ffmpeg y
ffprobe; codifica con x265, así que no necesita un hardware concreto. La CI corre
lo mismo en cada push y pull request; empujar un tag `v*` compila los binarios y
publica un release.

## Limitaciones

- La SRT es texto plano: se pierden colores, fuentes, cursivas, cajas, efectos y
  posiciones exactas. Un cartel va arriba o abajo según la mitad de la imagen en
  que está colocado, sube por el relleno de `--lines`, no hasta su posición
  real, y siempre centrado. Si los carteles molestan, se quitan con
  `--exclude-style`, o se queman con `--burn`.
- El karaoke generado cuyas líneas originales no están en el archivo queda sin
  letra: rehacerla desde las letras obligaría a adivinar los espacios.
- No se usa `{\an8}` ni otra marca de posición: no son parte del estándar SRT y
  el reproductor de Samsung las muestra como texto, tapando además el diálogo
  que coincide con ellas.
- El relleno de `--lines` depende del tamaño de letra del reproductor. Está
  calibrado para el de Samsung con subtítulos «Pequeño»; con otro tamaño o en
  otro reproductor puede quedar más alto o más bajo.
- Solo convierte pistas ASS/SSA. Los subtítulos de imagen (PGS, VobSub) y las
  SRT que ya trae el MKV se copian tal cual. `--burn` también quema solo pistas
  ASS/SSA.
- `--burn` recodifica: siempre hay algo de pérdida, invisible con la calidad por
  defecto, y lleva tiempo (Quick Sync, unos 7 minutos por episodio; x265 en la
  CPU, horas por episodio). Solo se usa Intel Quick Sync como codificador por
  hardware: NVIDIA, AMD y Apple (VideoToolbox) aún no se han medido, así que en un
  Mac `auto` recurre a x265. Comprobado solo con originales SDR (BT.709).
- Sin `--burn` no recodifica: si el TV no soporta el vídeo o el audio del
  archivo, `subflat` no lo arregla. Con él, el vídeo sale en HEVC 10 bit, que el
  TV tiene que soportar; el audio se sigue copiando tal cual.
- Comprobado con el reproductor nativo de un Samsung QN65Q65D (firmware
  T-NKLDUABC-0090-2123.9), desde USB: muestra todas las líneas del subtítulo
  fusionado, y con el relleno el cartel arriba y el diálogo abajo. Otros modelos
  y las apps de terceros (Plex, Jellyfin) pueden comportarse distinto.

## Contribuir

Los issues y pull requests son bienvenidos. Antes de abrir un PR, pasa
`go vet ./...` y `go test ./...` con MKVToolNix y ffmpeg instalados para que
corran también los tests de integración. Los commits siguen
[Conventional Commits](https://www.conventionalcommits.org/).

## Licencia

MIT
