package main

import (
	"os"
	"reflect"
	"testing"

	"github.com/0magnet/lolcat-go/lol"
)

func parseArgs(t *testing.T, argv ...string) (lol.Options, bool, []string, int) {
	t.Helper()
	o := lol.DefaultOptions()
	force := false
	files, code := parse(argv, &o, &force)
	return o, force, files, code
}

func TestParseLongAndShortForms(t *testing.T) {
	cases := []struct {
		name  string
		argv  []string
		check func(lol.Options, bool, []string) error
	}{
		{"defaults", nil, func(o lol.Options, f bool, files []string) error {
			if o.Spread != 3.0 || o.Freq != 0.1 || o.Seed != 0 || o.Duration != 12 || o.Speed != 20.0 {
				t.Errorf("defaults drifted: %+v", o)
			}
			return nil
		}},
		{"separate short values", []string{"-S", "42", "-p", "2.5", "-F", "0.3"}, func(o lol.Options, f bool, files []string) error {
			if o.Seed != 42 || o.Spread != 2.5 || o.Freq != 0.3 {
				t.Errorf("got %+v", o)
			}
			return nil
		}},
		{"long with equals", []string{"--seed=42", "--spread=2.5", "--freq=0.3"}, func(o lol.Options, f bool, files []string) error {
			if o.Seed != 42 || o.Spread != 2.5 || o.Freq != 0.3 {
				t.Errorf("got %+v", o)
			}
			return nil
		}},
		{"long with space", []string{"--seed", "42", "--duration", "3", "--speed", "5"}, func(o lol.Options, f bool, files []string) error {
			if o.Seed != 42 || o.Duration != 3 || o.Speed != 5 {
				t.Errorf("got %+v", o)
			}
			return nil
		}},
		{"bundled short flags", []string{"-fait"}, func(o lol.Options, f bool, files []string) error {
			if !f || !o.Animate || !o.Invert || !o.Truecolor {
				t.Errorf("bundle not applied: force=%v %+v", f, o)
			}
			return nil
		}},
		{"bundle ending in a value option", []string{"-fS", "7"}, func(o lol.Options, f bool, files []string) error {
			if !f || o.Seed != 7 {
				t.Errorf("got force=%v %+v", f, o)
			}
			return nil
		}},
		{"files and dash", []string{"-f", "a.txt", "-", "b.txt"}, func(o lol.Options, f bool, files []string) error {
			if !reflect.DeepEqual(files, []string{"a.txt", "-", "b.txt"}) {
				t.Errorf("files = %q", files)
			}
			return nil
		}},
		{"double dash stops option parsing", []string{"-f", "--", "-p", "--spread"}, func(o lol.Options, f bool, files []string) error {
			if !reflect.DeepEqual(files, []string{"-p", "--spread"}) {
				t.Errorf("files = %q", files)
			}
			if o.Spread != 3.0 {
				t.Errorf("spread should be untouched, got %v", o.Spread)
			}
			return nil
		}},
		{"trailing option with no value is ignored", []string{"-f", "-p"}, func(o lol.Options, f bool, files []string) error {
			// This is what the original does rather than complaining.
			if o.Spread != 3.0 {
				t.Errorf("spread = %v, want the default", o.Spread)
			}
			return nil
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o, force, files, code := parseArgs(t, c.argv...)
			if code >= 0 {
				t.Fatalf("parse stopped with code %d", code)
			}
			_ = c.check(o, force, files)
		})
	}
}

func TestParseRejectsBadValues(t *testing.T) {
	// die writes to stderr; keep the test output clean.
	old := os.Stderr
	devnull, _ := os.Open(os.DevNull)
	os.Stderr = devnull
	defer func() { os.Stderr = old; devnull.Close() }()

	for _, argv := range [][]string{
		{"-p", "abc"},
		{"-S", "1.5"},
		{"-d", "abc"},
		{"--bogus"},
		{"-Z"},
	} {
		if _, _, _, code := parseArgs(t, argv...); code != 255 {
			t.Errorf("parse(%q) = %d, want 255", argv, code)
		}
	}
}

func TestOpenErrorMessages(t *testing.T) {
	_, err := os.Open("/definitely/not/here")
	if got := openError("/definitely/not/here", err); got != "lolcat: /definitely/not/here: No such file or directory" {
		t.Errorf("got %q", got)
	}
}
