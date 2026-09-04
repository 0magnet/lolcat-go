# lolcat-go

A Go port of [lolcat](https://github.com/busyloop/lolcat) 100.0.1 by Moe
(moe@busyloop.net). It concatenates files, or standard input, to standard
output, painted with a travelling rainbow.

The port is **byte-exact**: for every input and option combination tested it
produces output identical to the Ruby original.

It has no dependencies. The Ruby version leans on the `paint` and `optimist`
gems; the parts of those it actually uses — the RGB-to-256 approximation and
the option grammar — are reimplemented here and diffed against the originals.

## Install

```
go install github.com/0magnet/lolcat-go/cmd/lolcat@latest
```

## Use

The command line matches the original:

```
fortune | lolcat
lolcat -a -d 20 -F 0.2 banner.txt
ls --color=always | lolcat -f | less -R
cowsay moo | lolcat -t -S 42
```

Run `lolcat --help` for the full option list. All of the original's options
are implemented: `-p/--spread`, `-F/--freq`, `-S/--seed`, `-a/--animate`,
`-d/--duration`, `-s/--speed`, `-i/--invert`, `-t/--truecolor`,
`-f/--force`, `-v/--version` and `-h/--help`. Files are read in order, `-`
means standard input, and with no file at all it reads standard input.

Like the original, it only colours when standard output is a terminal. Pipe
it somewhere and it copies through untouched unless you pass `-f`.

## Library

```go
import "github.com/0magnet/lolcat-go/lol"

o := lol.DefaultOptions()
o.Seed, o.OS, o.Truecolor = 42, 42, true
fmt.Print(lol.String("hello world\n", o))
```

For streams, `lol.Cat` takes an `io.Reader` and writes to any `io.Writer`:

```go
c := &lol.Cat{Opts: o, Out: os.Stdout, TTY: true}
c.SetMode(os.Getenv("COLORTERM"))
err := c.Cat(os.Stdin)
```

`Rainbow`, `RainbowHex` and `ColorSeq` are exported if you only want the
gradient and not the plumbing.

## Verification

`lolcat-go` was diffed against lolcat 100.0.1 running under Ruby 3.4, with
`paint` 2.3.0 and `optimist` 3.2.1:

| Suite | Comparisons | Result |
|---|---|---|
| Corpus × 11 option sets × 4 `COLORTERM` values | 704 | all identical |
| Fuzz seeds 500–900 × 6 option sets × 2 `COLORTERM` values | 4,812 | all identical |
| CLI behaviour: multiple files, `-`, `--`, bundled flags, animation, `-v`, `-h`, bad options, missing file, directory | 20 | all identical |

The corpus covers tabs, blank lines, missing trailing newlines, UTF-8, input
that already contains SGR codes, OSC 8 hyperlinks, charset-selection escapes,
control bytes, and lines long enough to cross the read window several times.
The fuzzer generates input biased towards escape sequences, including
truncated ones.

## Notes on fidelity

The rainbow itself is three sines 120° apart, but a few details of the Ruby
implementation are load-bearing, and getting any of them wrong changes the
output:

- The channels are rendered with `"%02X"`, and Ruby truncates a Float on the
  way to an integer. Rounding instead shifts about half of all colours by one.
- Ruby's `String#scan` yields one extra empty match at the end of the string,
  which Go's `FindAll` drops. That match is printed — as a colour change with
  no character after it, visible at the end of every line — and it counts
  towards the offset the next line starts from.
- Input is read in 4096-byte windows, and a line longer than one window is
  painted in pieces. The original saves and restores the offset around the
  seam, so the colours continue across it but the following line still starts
  where it would have.
- A stream that ends in the middle of an escape sequence has its whole last
  buffer discarded: `printf 'hello\033[' | lolcat -f` prints nothing at all.
- The 256-colour approximation walks a threshold upwards in steps of 42.5 to
  decide whether a colour is grey, and divides by 256 rather than 255 for the
  6×6×6 cube. Both are easy to "correct" into different colours.
- Colour depth comes from `COLORTERM` alone — `truecolor` or `24bit` — and
  not from paint's fuller terminal probe, which is why lolcat is more
  conservative than paint about 24-bit colour.
- A tab becomes eight spaces, each of which takes its own colour, so a tab
  advances the gradient by eight steps.

## Deliberate differences

- **Dependencies.** There are none. `paint` and `optimist` are reimplemented
  in `lol` and `cmd/lolcat` respectively, matched against the gems rather
  than reinvented.
- **Broken pipe.** `lolcat big | head` ends on Go's default `SIGPIPE`
  handling (status 141) where Ruby exits 1. Both are silent.
- **Random seeds.** With `-S 0`, or no `-S` at all, the offset is picked by
  Go's `math/rand` rather than Ruby's, so an unseeded run will not reproduce
  a particular Ruby run. Both are random by design; pass `-S` for a fixed
  gradient.
- **Long options are not abbreviated.** `--spread` works, `--spr` does not.

## Licence

BSD 3-Clause, inherited from lolcat. See `LICENSE`.

Original lolcat is copyright © 2016 Moe <moe@busyloop.net>,
<https://github.com/busyloop/lolcat/>. This port carries that licence
unchanged; it is not relicensed.

## Dependency Graph

Made with [goda](https://github.com/loov/goda):

```
go run github.com/loov/goda@latest graph github.com/0magnet/lolcat-go/... | dot -Tsvg -o docs/lolcat-go-goda-graph.svg
```

![Dependency Graph](docs/lolcat-go-goda-graph.svg "github.com/0magnet/lolcat-go Dependency Graph")

## Lines of Code

Made with [gocloc](https://github.com/hhatto/gocloc) (excludes `vendor/`, `node_modules/`, `.git/`):

```
gocloc --not-match-d='(vendor|node_modules|\.git)' .
```

```
-------------------------------------------------------------------------------
Language                     files          blank        comment           code
-------------------------------------------------------------------------------
Go                               5            106            154            933
Markdown                         1             36              0            110
-------------------------------------------------------------------------------
TOTAL                            6            142            154           1043
-------------------------------------------------------------------------------
```
