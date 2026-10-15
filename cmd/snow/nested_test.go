package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestNestedSnowRefused: snow and snow <name> refuse inside a pane, exit 1,
// and say how to leave or force it.
func TestNestedSnowRefused(t *testing.T) {
	t.Setenv("SNOW", "/run/snow/default.sock,7")
	for _, args := range [][]string{nil, {"work"}} {
		var out, errOut bytes.Buffer
		code := run(args, &out, &errOut)
		msg := errOut.String()
		if code != 1 || out.Len() != 0 {
			t.Fatalf("snow %q: code %d, stdout %q", args, code, out.String())
		}
		for _, want := range []string{"already inside a snow session (pane 7)", "Ctrl+b d", unsetHint()} {
			if !strings.Contains(msg, want) {
				t.Errorf("snow %q: stderr %q lacks %q", args, msg, want)
			}
		}
	}
}

// TestNestedSnowEmptyValueIsUnset: an empty SNOW does not block a session,
// so `SNOW= snow` also works as an override.
func TestNestedSnowEmptyValueIsUnset(t *testing.T) {
	t.Setenv("SNOW", "")
	if nested() != nil {
		t.Fatal("empty SNOW counted as nested")
	}
}

// TestVersionInsidePane: snow version and snow help work inside a pane.
func TestVersionInsidePane(t *testing.T) {
	t.Setenv("SNOW", "/run/snow/default.sock,7")
	var out, errOut bytes.Buffer
	if code := run([]string{"version"}, &out, &errOut); code != 0 || out.String() != "snow dev\n" {
		t.Fatalf("version: code %d, stdout %q, stderr %q", code, out.String(), errOut.String())
	}
	out.Reset()
	if code := run([]string{"help"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "usage:") {
		t.Fatalf("help: code %d, stdout %q", code, out.String())
	}
}
