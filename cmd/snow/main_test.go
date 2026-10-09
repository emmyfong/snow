package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		args []string
		want command
		ok   bool
	}{
		{nil, command{kind: "attach"}, true},
		{[]string{"work"}, command{kind: "attach", session: "work"}, true},
		{[]string{"version"}, command{kind: "version"}, true},
		{[]string{"help"}, command{kind: "help"}, true},
		{[]string{"--help"}, command{kind: "help"}, true},
		{[]string{"server"}, command{kind: "server"}, true},
		{[]string{"-x"}, command{}, false},
		{[]string{"a/b"}, command{}, false},
		{[]string{"a", "b"}, command{}, false},
	}
	for _, tt := range tests {
		got, ok := parseArgs(tt.args)
		if got != tt.want || ok != tt.ok {
			t.Errorf("parseArgs(%q) = %+v, %v; want %+v, %v", tt.args, got, ok, tt.want, tt.ok)
		}
	}
}

func TestRunVersionAndUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"version"}, &out, &errOut); code != 0 || out.String() != "snow dev\n" {
		t.Fatalf("version: code %d, output %q", code, out.String())
	}
	out.Reset()
	if code := run([]string{"a", "b"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "usage:") {
		t.Fatalf("bad args: code %d, stderr %q", code, errOut.String())
	}
}
