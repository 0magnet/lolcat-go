// Command lolcat concatenates files, or standard input, to standard output,
// painted with a rainbow.
//
// It is the CLI half of the port of https://github.com/busyloop/lolcat by
// moe@busyloop.net, and takes the same options:
//
//	fortune | lolcat
//	lolcat -a -d 20 -F 0.2 banner.txt
//	ls --color=always | lolcat -f | less -R
package main

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/0magnet/lolcat-go/lol"
)

// Version is the upstream release this was ported from. lolcat's own
// versions jumped to 100.x, so this is not a typo.
const Version = "100.0.1"

const versionLine = "lolcat " + Version + " (c)2011 moe@busyloop.net"

// The help text, byte for byte what the original's option parser prints.
const helpText = `
Usage: lolcat [OPTION]... [FILE]...

Concatenate FILE(s), or standard input, to standard output.
With no FILE, or when FILE is -, read standard input.

  -p, --spread=<f>      Rainbow spread (default: 3.0)
  -F, --freq=<f>        Rainbow frequency (default: 0.1)
  -S, --seed=<i>        Rainbow seed, 0 = random (default: 0)
  -a, --animate         Enable psychedelics
  -d, --duration=<i>    Animation duration (default: 12)
  -s, --speed=<f>       Animation speed (default: 20.0)
  -i, --invert          Invert fg and bg
  -t, --truecolor       24-bit (truecolor)
  -f, --force           Force color even when stdout is not a tty
  -v, --version         Print version and exit
  -h, --help            Show this message

Examples:
  lolcat f - g      Output f's contents, then stdin, then g's contents.
  lolcat            Copy standard input to standard output.
  fortune | lolcat  Display a rainbow cookie.

Report lolcat bugs to <https://github.com/busyloop/lolcat/issues>
lolcat home page: <https://github.com/busyloop/lolcat/>
Report lolcat translation bugs to <http://speaklolcat.com/>
`

func main() {
	os.Exit(run(os.Args[1:]))
}

// die reproduces the original option parser's failure: a message on stderr
// and exit status 255 (Ruby's exit(-1)).
func die(format string, a ...any) int {
	fmt.Fprintf(os.Stderr, "Error: "+format+".\nTry --help for help.\n", a...)
	return 255
}

func run(argv []string) int {
	opts := lol.DefaultOptions()
	force := false

	files, code := parse(argv, &opts, &force)
	if code >= 0 {
		return code
	}

	if opts.Spread < 0.1 {
		return die("argument --spread must be >= 0.1")
	}
	if float64(opts.Duration) < 0.1 {
		return die("argument --duration must be >= 0.1")
	}
	if opts.Speed < 0.1 {
		return die("argument --speed must be >= 0.1")
	}

	opts.OS = float64(opts.Seed)
	if opts.OS == 0 {
		// Colors, not secrets: a predictable rainbow offset is the feature.
		opts.OS = float64(rand.Intn(256)) //nolint:gosec
	}

	stdoutTTY := isTTY(os.Stdout)
	if len(files) == 0 {
		files = []string{"-"}
	}

	// The original swallows Interrupt, and its ensure block still restores
	// the terminal on the way out. Without this, Ctrl+C during an animation
	// would leave the cursor hidden.
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt)
	go func() {
		<-interrupted
		if stdoutTTY {
			os.Stdout.WriteString("\x1b[m\x1b[?25h\x1b[?1;5;2004l") //nolint:errcheck,gosec
		}
		os.Exit(0)
	}()

	for _, name := range files {
		in := io.Reader(os.Stdin)
		inTTY := isTTY(os.Stdin)
		if name != "-" {
			// Opening the file named on the command line is the whole job.
			f, err := os.Open(name) //nolint:gosec
			if err != nil {
				fmt.Println(openError(name, err))
				return 1
			}
			if st, serr := f.Stat(); serr == nil && st.IsDir() {
				f.Close() //nolint:errcheck,gosec
				fmt.Printf("lolcat: %s: Is a directory\n", name)
				return 1
			}
			defer f.Close() //nolint:errcheck,gosec
			in, inTTY = f, false
		}

		if stdoutTTY || force {
			c := &lol.Cat{Opts: opts, Out: os.Stdout, TTY: stdoutTTY}
			c.SetMode(os.Getenv("COLORTERM"))
			if err := c.Cat(in); err != nil {
				return 1
			}
			continue
		}

		// Not a terminal and not forced: copy through untouched. The
		// original reads a terminal line by line so that it stays
		// interactive, and splices anything else.
		if inTTY {
			br := newLineReader(in)
			for {
				line, err := br()
				if line != "" {
					os.Stdout.WriteString(line) //nolint:errcheck,gosec
				}
				if err != nil {
					break
				}
			}
		} else if _, err := io.Copy(os.Stdout, in); err != nil {
			return 1
		}
	}
	return 0
}

// parse walks argv the way the original's option parser does: long options
// with an optional "=value", short options that may be bundled ("-fi"), "--"
// to stop, and "-" as a filename. It returns the file arguments, and an exit
// code of 0 or more if the program should stop now.
func parse(argv []string, opts *lol.Options, force *bool) ([]string, int) {
	var files []string

	i := 0

	// next takes the following argument as a value. An option that wants one
	// but sits at the end of argv is ignored, which is what the original
	// does — it leaves the default in place and carries on.
	next := func() (string, bool) {
		if i+1 < len(argv) {
			i++
			return argv[i], true
		}
		return "", false
	}

	for ; i < len(argv); i++ {
		arg := argv[i]

		switch {
		case arg == "--":
			return append(files, argv[i+1:]...), -1

		case arg == "-" || !strings.HasPrefix(arg, "-"):
			files = append(files, arg)

		case strings.HasPrefix(arg, "--"):
			name := arg[2:]
			get := next
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				inline := name[eq+1:]
				name = name[:eq]
				get = func() (string, bool) { return inline, true }
			}
			if code := setLong(name, arg, get, opts, force); code >= 0 {
				return nil, code
			}

		default:
			// Short options may be bundled: "-fi" is "-f -i", and the last
			// one in the bundle may still take the next argument.
			for _, r := range arg[1:] {
				if code := setShort(r, next, opts, force); code >= 0 {
					return nil, code
				}
			}
		}
	}
	return files, -1
}

// setLong applies one long option. It returns -1 to carry on, or an exit code.
func setLong(name, raw string, get func() (string, bool), opts *lol.Options, force *bool) int {
	switch name {
	case "spread":
		return setFloat("spread", get, &opts.Spread)
	case "freq":
		return setFloat("freq", get, &opts.Freq)
	case "seed":
		return setInt("seed", get, &opts.Seed)
	case "duration":
		return setInt("duration", get, &opts.Duration)
	case "speed":
		return setFloat("speed", get, &opts.Speed)
	case "animate":
		opts.Animate = true
	case "invert":
		opts.Invert = true
	case "truecolor":
		opts.Truecolor = true
	case "force":
		*force = true
	case "version":
		fmt.Println(versionLine)
		return 0
	case "help":
		return showHelp()
	default:
		return die("unknown argument '%s'", raw)
	}
	return -1
}

// setShort applies one short option, possibly one of several bundled together.
func setShort(r rune, get func() (string, bool), opts *lol.Options, force *bool) int {
	switch r {
	case 'p':
		return setFloat("spread", get, &opts.Spread)
	case 'F':
		return setFloat("freq", get, &opts.Freq)
	case 'S':
		return setInt("seed", get, &opts.Seed)
	case 'd':
		return setInt("duration", get, &opts.Duration)
	case 's':
		return setFloat("speed", get, &opts.Speed)
	case 'a':
		opts.Animate = true
	case 'i':
		opts.Invert = true
	case 't':
		opts.Truecolor = true
	case 'f':
		*force = true
	case 'v':
		fmt.Println(versionLine)
		return 0
	case 'h':
		return showHelp()
	default:
		return die("unknown argument '-%c'", r)
	}
	return -1
}

func setFloat(name string, get func() (string, bool), dst *float64) int {
	s, ok := get()
	if !ok {
		return -1
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return die("option '%s' needs a floating-point number", name)
	}
	*dst = v
	return -1
}

func setInt(name string, get func() (string, bool), dst *int) int {
	s, ok := get()
	if !ok {
		return -1
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return die("option '%s' needs an integer", name)
	}
	*dst = v
	return -1
}

// showHelp prints the usage through the rainbow, as the original does, and
// exits 1.
func showHelp() int {
	opts := lol.Options{
		Spread:   8.0,
		Freq:     0.3,
		Duration: 12,
		Speed:    20.0,
		OS:       rand.Float64() * 8192, //nolint:gosec // colors, not secrets
	}
	c := &lol.Cat{Opts: opts, Out: os.Stdout, TTY: isTTY(os.Stdout)}
	c.SetMode(os.Getenv("COLORTERM"))
	c.Cat(strings.NewReader(helpText)) //nolint:errcheck,gosec // help text to stdout; a failure here has nowhere to go
	fmt.Println()
	return 1
}

func openError(name string, err error) string {
	switch {
	case os.IsNotExist(err):
		return fmt.Sprintf("lolcat: %s: No such file or directory", name)
	case os.IsPermission(err):
		return fmt.Sprintf("lolcat: %s: Permission denied", name)
	default:
		return fmt.Sprintf("lolcat: %s: %v", name, err)
	}
}

func isTTY(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// newLineReader returns a function yielding one line at a time, used for the
// uncolored passthrough of a terminal.
func newLineReader(r io.Reader) func() (string, error) {
	buf := make([]byte, 1)
	var sb strings.Builder
	return func() (string, error) {
		sb.Reset()
		for {
			n, err := r.Read(buf)
			if n > 0 {
				sb.WriteByte(buf[0])
				if buf[0] == '\n' {
					return sb.String(), nil
				}
			}
			if err != nil {
				return sb.String(), err
			}
		}
	}
}
