package lol

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

// Every expected value in this file was captured from lolcat 100.0.1 running
// under Ruby, so a change in the gradient, in the 256-color rounding or in
// the offset bookkeeping fails the test rather than quietly redefining what
// "correct" means.

func opts(seed int, mut ...func(*Options)) Options {
	o := DefaultOptions()
	o.Seed = seed
	o.OS = float64(seed)
	for _, f := range mut {
		f(&o)
	}
	return o
}

func TestRainbowHex(t *testing.T) {
	// Lol.rainbow(0.1, i), evaluated in Ruby.
	cases := []struct {
		i    float64
		want string
	}{
		{0, "#80ED12"},
		{1, "#8CE70C"},
		{2, "#99DF07"},
		{3, "#A5D604"},
		{10, "#EA850F"},
		{42, "#1181ED"},
		{42.5, "#0E87E9"},
		{43, "#0B8EE6"},
		{100, "#3A46FE"},
	}
	for _, c := range cases {
		if got := RainbowHex(0.1, c.i); got != c.want {
			t.Errorf("RainbowHex(0.1, %v) = %s, want %s", c.i, got, c.want)
		}
	}
}

func TestRainbowChannelsAreTruncated(t *testing.T) {
	// The original renders the channels with "%02X", and Ruby truncates a
	// Float on the way to an integer. Rounding instead shifts colors by one
	// on roughly half of all positions, so this pins the direction.
	// Green at i=0 is 237.98, which Ruby renders as ED, not EE.
	if _, g, _ := Rainbow(0.1, 0); g != 0xED {
		t.Errorf("green at i=0 = %d, want 237 (truncated, not rounded)", g)
	}
	r, g, b := Rainbow(0.1, 43)
	if r != 0x0B || g != 0x8E || b != 0xE6 {
		t.Errorf("Rainbow(0.1, 43) = %d,%d,%d, want 11,142,230", r, g, b)
	}
	// sin never reaches 1 exactly here, so the channels stay inside 1..255.
	for i := 0.0; i < 500; i += 0.25 {
		r, g, b := Rainbow(0.1, i)
		for _, c := range []int{r, g, b} {
			if c < 0 || c > 255 {
				t.Fatalf("channel %d out of range at i=%v", c, i)
			}
		}
	}
}

func TestColorSeq256(t *testing.T) {
	// Taken from Paint.color with Paint.mode = 256. The greyscale ramp and
	// the 6x6x6 cube meet awkwardly, so the boundaries are pinned.
	cases := []struct {
		r, g, b int
		want    string
	}{
		{0, 0, 0, "\x1b[38;5;232m"},
		{10, 10, 10, "\x1b[38;5;233m"},
		{42, 42, 42, "\x1b[38;5;236m"},
		{43, 43, 43, "\x1b[38;5;236m"},
		{85, 85, 86, "\x1b[38;5;240m"},
		{128, 130, 127, "\x1b[38;5;144m"},
		{200, 200, 200, "\x1b[38;5;250m"},
		{255, 255, 255, "\x1b[38;5;255m"},
		{255, 0, 0, "\x1b[38;5;196m"},
		{0, 255, 0, "\x1b[38;5;46m"},
		{1, 2, 3, "\x1b[38;5;232m"},
		{100, 50, 200, "\x1b[38;5;98m"},
		{43, 10, 10, "\x1b[38;5;52m"},
	}
	for _, c := range cases {
		if got := ColorSeq(c.r, c.g, c.b, Mode256, false); got != c.want {
			t.Errorf("ColorSeq(%d,%d,%d) = %q, want %q", c.r, c.g, c.b, got, c.want)
		}
	}
}

func TestColorSeqBackground(t *testing.T) {
	if got := ColorSeq(255, 0, 0, Mode256, true); got != "\x1b[48;5;196m" {
		t.Errorf("background 256 = %q", got)
	}
	if got := ColorSeq(11, 142, 230, ModeTrueColor, true); got != "\x1b[48;2;11;142;230m" {
		t.Errorf("background truecolor = %q", got)
	}
}

func TestDetectMode(t *testing.T) {
	// lolcat trusts COLORTERM and nothing else.
	for _, s := range []string{"truecolor", "24bit"} {
		if DetectMode(s) != ModeTrueColor {
			t.Errorf("DetectMode(%q) should be truecolor", s)
		}
	}
	for _, s := range []string{"", "256color", "TRUECOLOR", "yes"} {
		if DetectMode(s) != Mode256 {
			t.Errorf("DetectMode(%q) should be 256", s)
		}
	}
}

func TestString256(t *testing.T) {
	got := String("hello world\n", opts(42))
	want := "\x1b[38;5;39mh\x1b[39m\x1b[38;5;39me\x1b[39m\x1b[38;5;39ml\x1b[39m" +
		"\x1b[38;5;39ml\x1b[39m\x1b[38;5;39mo\x1b[39m\x1b[38;5;39m \x1b[39m" +
		"\x1b[38;5;38mw\x1b[39m\x1b[38;5;38mo\x1b[39m\x1b[38;5;44mr\x1b[39m" +
		"\x1b[38;5;44ml\x1b[39m\x1b[38;5;44md\x1b[39m\x1b[38;5;44m\x1b[39m\n"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestStringTrueColor(t *testing.T) {
	got := String("hello world\n", opts(42, func(o *Options) { o.Truecolor = true }))
	want := "\x1b[38;2;11;142;230mh\x1b[39m\x1b[38;2;10;146;227me\x1b[39m" +
		"\x1b[38;2;8;150;225ml\x1b[39m\x1b[38;2;7;154;222ml\x1b[39m" +
		"\x1b[38;2;5;158;219mo\x1b[39m\x1b[38;2;4;162;216m \x1b[39m" +
		"\x1b[38;2;3;166;213mw\x1b[39m\x1b[38;2;3;170;210mo\x1b[39m" +
		"\x1b[38;2;2;174;206mr\x1b[39m\x1b[38;2;1;178;203ml\x1b[39m" +
		"\x1b[38;2;1;182;199md\x1b[39m\x1b[38;2;1;186;196m\x1b[39m\n"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestInvertPaintsTheBackground(t *testing.T) {
	got := String("ab\n", opts(42, func(o *Options) { o.Invert = true }))
	want := "\x1b[48;5;39ma\x1b[49m\x1b[48;5;39mb\x1b[49m\x1b[48;5;39m\x1b[49m\n"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestSpreadAndFreq(t *testing.T) {
	o := opts(7, func(o *Options) { o.Spread, o.Freq, o.Truecolor = 1, 0.5, true })
	got := String("abcdef\n", o)
	want := "\x1b[38;2;31;104;247ma\x1b[39m\x1b[38;2;3;166;213mb\x1b[39m" +
		"\x1b[38;2;6;220;157mc\x1b[39m\x1b[38;2;38;250;94md\x1b[39m" +
		"\x1b[38;2;92;251;40me\x1b[39m\x1b[38;2;155;221;6mf\x1b[39m" +
		"\x1b[38;2;211;169;3m\x1b[39m\n"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestTabsBecomeEightSpaces(t *testing.T) {
	got := String("a\tb\n", opts(1, func(o *Options) { o.Truecolor = true }))
	if strings.Contains(got, "\t") {
		t.Fatal("tab survived")
	}
	// Each of the eight spaces gets its own color, so the tab advances the
	// gradient by eight steps rather than one.
	want := "\x1b[38;2;153;223;7ma\x1b[39m\x1b[38;2;157;220;6m \x1b[39m" +
		"\x1b[38;2;161;217;5m \x1b[39m\x1b[38;2;165;214;4m \x1b[39m" +
		"\x1b[38;2;169;211;3m \x1b[39m\x1b[38;2;173;207;2m \x1b[39m" +
		"\x1b[38;2;177;204;1m \x1b[39m\x1b[38;2;181;201;1m \x1b[39m" +
		"\x1b[38;2;185;197;1m \x1b[39m\x1b[38;2;188;194;1mb\x1b[39m" +
		"\x1b[38;2;192;190;1m\x1b[39m\n"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestExistingEscapesPassThrough(t *testing.T) {
	// The input's own SGR codes are copied out ahead of the rainbow color,
	// and they do not consume a gradient step.
	got := String("\x1b[31mR\x1b[0m\n", opts(1, func(o *Options) { o.Truecolor = true }))
	want := "\x1b[31m\x1b[38;2;153;223;7mR\x1b[39m" +
		"\x1b[0m\x1b[38;2;157;220;6m\x1b[39m" +
		"\x1b[38;2;161;217;5m\x1b[39m\n"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestUTF8IsColouredPerRune(t *testing.T) {
	got := String("héllo ✓\n", opts(3, func(o *Options) { o.Truecolor = true }))
	want := "\x1b[38;2;177;204;1mh\x1b[39m\x1b[38;2;181;201;1mé\x1b[39m" +
		"\x1b[38;2;185;197;1ml\x1b[39m\x1b[38;2;188;194;1ml\x1b[39m" +
		"\x1b[38;2;192;190;1mo\x1b[39m\x1b[38;2;196;186;1m \x1b[39m" +
		"\x1b[38;2;199;182;1m✓\x1b[39m\x1b[38;2;203;179;1m\x1b[39m\n"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestNoTrailingNewline(t *testing.T) {
	got := String("ab", opts(1, func(o *Options) { o.Truecolor = true }))
	want := "\x1b[38;2;153;223;7ma\x1b[39m\x1b[38;2;157;220;6mb\x1b[39m" +
		"\x1b[38;2;161;217;5m\x1b[39m"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestBlankLineStillEmitsAColour(t *testing.T) {
	// An empty line is one empty match, which prints a bare color change.
	got := String("\n", opts(1, func(o *Options) { o.Truecolor = true }))
	want := "\x1b[38;2;153;223;7m\x1b[39m\n"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestIncompleteEscapeAtEOFIsDropped(t *testing.T) {
	// A stream that ends mid-escape leaves the original waiting for the rest
	// of the sequence; it hits EOF and throws the buffer away. Nothing is
	// printed, not even the text before the escape.
	if got := String("hello\x1b[", opts(1)); got != "" {
		t.Errorf("got %q, want empty", got)
	}
	// One byte earlier, the same input prints normally.
	if got := String("hello", opts(1)); got == "" {
		t.Error("complete input printed nothing")
	}
}

func TestGradientRestartsPerStream(t *testing.T) {
	// lolcat resets the offset for every file argument, so two files get the
	// same colors rather than a continuous rainbow.
	a := String("hello\n", opts(42))
	b := String("hello\n", opts(42))
	if a != b {
		t.Error("two streams with the same seed differ")
	}
	// Within one stream the second line does move on.
	two := String("hello\nhello\n", opts(42))
	if two == a+a {
		t.Error("the second line of a stream should not repeat the first")
	}
}

// The read window is 4096 bytes, and a line longer than that is painted in
// pieces with the offset carried across the seam. It is the fiddliest part of
// the original, so these pin whole outputs by hash.
func TestReadWindowSeam(t *testing.T) {
	long := strings.Repeat("0123456789", 600) + "\n"
	veryLong := strings.Repeat("abcdefghij", 2000) + "\ntail line one\ntail line two\n"
	escWindow := strings.Repeat("\x1b[31mab\x1b[39m", 680) + "\n"

	cases := []struct {
		name string
		in   string
		o    Options
		want string
	}{
		{
			"long line, one seam",
			long,
			opts(11, func(o *Options) { o.Spread, o.Freq = 2.5, 0.15 }),
			"4ec70910b1600dba39aecea18dbfd3160560a0246abe27343253d1013f512f31",
		},
		{
			"very long line, several seams, then short lines",
			veryLong,
			opts(11, func(o *Options) { o.Spread, o.Freq, o.Truecolor = 2.5, 0.15, true }),
			"f73157e902d19a51cd272b7d72fa5197b38f7fbf24c57952cb8b51c74a91ffe9",
		},
		{
			"escape sequences straddling the window",
			escWindow,
			opts(11, func(o *Options) { o.Spread = 3 }),
			"67730cde2044ad9721ee5bfa82971dfa8fbe20395fefd2bdc33fe84702cfd9ef",
		},
	}
	for _, c := range cases {
		sum := sha256.Sum256([]byte(String(c.in, c.o)))
		if got := hex.EncodeToString(sum[:]); got != c.want {
			t.Errorf("%s: sha256 = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestScanMatchesRubyScan(t *testing.T) {
	// Ruby's String#scan ends with an extra empty pair that Go's FindAll
	// drops. The pair is printed and it counts towards the offset, so the
	// count has to match exactly.
	cases := []struct {
		in   string
		want int
	}{
		{"", 1},
		{"ab", 3},
		{"a\x1b[31mb", 3},
		{"héllo", 6},
		{"\x1b[31m", 2},
		{"a\x1b]8;;u\ab", 4},
		{"\x1b(Bx", 2},
	}
	for _, c := range cases {
		if got := len(scan(c.in)); got != c.want {
			t.Errorf("scan(%q) gave %d pairs, want %d", c.in, got, c.want)
		}
	}
}

func TestAnimateRedrawsInPlace(t *testing.T) {
	var b strings.Builder
	o := opts(42, func(o *Options) { o.Animate, o.Duration, o.Truecolor = true, 3, true })
	c := &Cat{Opts: o, Out: &b, Sleep: func(time.Duration) {}}
	c.SetMode("")
	if err := c.Cat(strings.NewReader("ab\n")); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	if !strings.HasPrefix(got, "\x1b[?25l\x1b7") {
		t.Errorf("animation should hide the cursor then save it: %q", got)
	}
	if n := strings.Count(got, "\x1b8"); n != 3 {
		t.Errorf("restored the cursor %d times, want one per frame (3)", n)
	}
	// Each frame advances by Spread, and the line's own newline is printed
	// once, after the last frame.
	if n := strings.Count(got, "\n"); n != 1 {
		t.Errorf("printed %d newlines, want 1", n)
	}
}
