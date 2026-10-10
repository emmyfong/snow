package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		args    []string
		want    command
		problem string // part of the error; "" means none
	}{
		{nil, command{kind: kindAttach}, ""},
		{[]string{"work"}, command{kind: kindAttach, session: "work"}, ""},
		{[]string{"version"}, command{kind: kindVersion}, ""},
		{[]string{"--version"}, command{kind: kindVersion}, ""},
		{[]string{"-v"}, command{kind: kindVersion}, ""},
		{[]string{"help"}, command{kind: kindHelp}, ""},
		{[]string{"--help"}, command{kind: kindHelp}, ""},
		{[]string{"server"}, command{kind: kindServer}, ""},
		{[]string{"-x"}, command{}, "unknown option -x"},
		{[]string{"a/b"}, command{}, "session names cannot contain"},
		{[]string{"a", "b"}, command{}, "too many arguments"},
	}
	for _, tt := range tests {
		got, err := parseArgs(tt.args)
		if got != tt.want || (err == nil) != (tt.problem == "") || (err != nil && !strings.Contains(err.Error(), tt.problem)) {
			t.Errorf("parseArgs(%q) = %+v, %v; want %+v, %q", tt.args, got, err, tt.want, tt.problem)
		}
	}
}

func TestRunVersionAndUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"version"}, &out, &errOut); code != 0 || out.String() != "snow dev\n" {
		t.Fatalf("version: code %d, output %q", code, out.String())
	}
	out.Reset()
	if code := run([]string{"a", "b"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "usage:") ||
		!strings.Contains(errOut.String(), "too many arguments") {
		t.Fatalf("bad args: code %d, stderr %q", code, errOut.String())
	}
}

func TestBadConfigStopsAttachAndNamesTheLine(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir reads this on Windows
	if err := os.MkdirAll(filepath.Join(home, ".snow"), 0o700); err != nil {
		t.Fatal(err)
	}
	body := "[scrollback]\nlines = 0\n"
	if err := os.WriteFile(filepath.Join(home, ".snow", "config.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"work"}, &out, &errOut); code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
	if got := errOut.String(); !strings.Contains(got, "line 2") || !strings.Contains(got, "scrollback.lines") {
		t.Fatalf("stderr %q, want the line and the key", got)
	}
}
