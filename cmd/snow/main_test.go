package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/transport"
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
		{[]string{"mcp"}, command{kind: kindMCP}, ""},
		{[]string{"auth", "status"}, command{kind: kindAuth, auth: authCommand{action: authStatus}}, ""},
		{[]string{"auth", "--help"}, command{kind: kindAuthHelp}, ""},
		{[]string{"auth"}, command{kind: kindAuthHelp}, "auth needs"},
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

// TestRunMCPOutsideSnow: outside a Snow pane, snow mcp speaks MCP, offers
// no tools, and says why on stderr.
func TestRunMCPOutsideSnow(t *testing.T) {
	t.Setenv(transport.EnvPane, "")
	t.Setenv(transport.EnvSocket, "")
	old := stdin
	stdin = strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n")
	t.Cleanup(func() { stdin = old })
	var out, errOut bytes.Buffer
	if code := run([]string{"mcp"}, &out, &errOut); code != 0 {
		t.Fatalf("code %d, stderr %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"tools":[]`) {
		t.Errorf("no empty tool list: %s", out.String())
	}
	if !strings.Contains(out.String(), transport.EnvPane) {
		t.Errorf("instructions do not say why: %s", out.String())
	}
	if !strings.Contains(errOut.String(), transport.EnvPane) {
		t.Errorf("stderr does not say why: %q", errOut.String())
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

// attachWithConfig runs snow with body as the user's config file and returns
// the exit code and what it printed to stderr.
func attachWithConfig(t *testing.T, body string) (int, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir reads this on Windows
	if err := os.MkdirAll(filepath.Join(home, ".snow"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".snow", "config.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := run([]string{"work"}, &out, &errOut)
	return code, errOut.String()
}

func TestBadConfigStopsAttachAndNamesTheLine(t *testing.T) {
	code, got := attachWithConfig(t, "[scrollback]\nlines = 0\n")
	if code != 1 || !strings.Contains(got, "line 2") || !strings.Contains(got, "scrollback.lines") {
		t.Fatalf("code %d, stderr %q, want 1 with the line and the key", code, got)
	}
}

// A binding the key table rejects is reported like any other config error:
// the file, the line, and the key.
func TestBadKeyBindingNamesTheFileAndLine(t *testing.T) {
	tests := []struct {
		name, body, line, key string
	}{
		{"unknown action", "[keys.bind]\n\"|\" = \"split-right\"\n\"%\" = \"split-sideways\"\n", "line 3", `keys.bind."%"`},
		{"bad key name", "[keys]\nprefix = \"C-a\"\n\n[keys.bind]\n\"Hyper\" = \"zoom\"\n", "line 5", `keys.bind.Hyper`},
		{"bad prefix", "# mine\n[keys]\nprefix = \"C-Nope\"\n", "line 3", "keys.prefix"},
	}
	for _, tt := range tests {
		code, got := attachWithConfig(t, tt.body)
		if code != 1 || !strings.Contains(got, "config.toml") || !strings.Contains(got, tt.line) || !strings.Contains(got, strconv.Quote(tt.key)) {
			t.Errorf("%s: code %d, stderr %q; want the file, %q, and %q", tt.name, code, got, tt.line, tt.key)
		}
	}
}
