package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/shell"
)

func TestParseClaude(t *testing.T) {
	tests := []struct {
		args    []string
		want    command
		problem string
	}{
		{[]string{"claude", "setup"}, command{kind: kindClaude, claude: claudeSetup}, ""},
		{[]string{"claude", "remove"}, command{kind: kindClaude, claude: claudeRemove}, ""},
		{[]string{"claude", "status"}, command{kind: kindClaude, claude: claudeStatus}, ""},
		{[]string{"claude", "help"}, command{kind: kindClaudeHelp}, ""},
		{[]string{"claude"}, command{kind: kindClaudeHelp}, "claude needs"},
		{[]string{"claude", "login"}, command{kind: kindClaudeHelp}, "unknown claude command"},
	}
	for _, tt := range tests {
		got, err := parseArgs(tt.args)
		if got != tt.want || (err == nil) != (tt.problem == "") || (err != nil && !strings.Contains(err.Error(), tt.problem)) {
			t.Errorf("parseArgs(%q) = %+v, %v; want %+v, %q", tt.args, got, err, tt.want, tt.problem)
		}
	}
}

// TestClaudeCommand: snow claude setup asks, writes the hooks to the file
// in CLAUDE_CONFIG_DIR, status reports them, and remove gives back the
// user's file.
func TestClaudeCommand(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	oldPath, oldDistros := snowPath, claudeDistros
	snowPath = func() (string, error) { return "/usr/local/bin/snow", nil }
	claudeDistros = func() ([]shell.Profile, error) { return nil, nil }
	t.Cleanup(func() { snowPath, claudeDistros = oldPath, oldDistros })
	path := filepath.Join(dir, "settings.json")
	mine := "{\n  \"model\": \"opus\"\n}\n"
	if err := os.WriteFile(path, []byte(mine), 0o600); err != nil {
		t.Fatal(err)
	}
	snow := func(answer string, args ...string) string {
		t.Helper()
		old := stdin
		stdin = strings.NewReader(answer)
		defer func() { stdin = old }()
		var out, errOut bytes.Buffer
		if code := run(args, &out, &errOut); code != 0 {
			t.Fatalf("snow %q: code %d, stderr %q", args, code, errOut.String())
		}
		return out.String()
	}
	if out := snow("n\n", "claude", "setup"); !strings.Contains(out, "Nothing changed") {
		t.Fatalf("setup without yes: %q", out)
	}
	if out := snow("", "claude", "status"); !strings.Contains(out, "not set") {
		t.Fatalf("status before setup: %q", out)
	}
	snow("y\n", "claude", "setup")
	if out := snow("", "claude", "status"); !strings.Contains(out, "are set") {
		t.Fatalf("status after setup: %q", out)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), `"'/usr/local/bin/snow' hook"`) || !strings.Contains(string(got), `"model": "opus"`) {
		t.Fatalf("settings after setup:\n%s", got)
	}
	snow("y\n", "claude", "remove")
	if got, _ := os.ReadFile(path); string(got) != mine {
		t.Fatalf("after remove:\n%s", got)
	}
}

// TestClaudeSetupRefusesOtherName: a snow run as snow-dev changes nothing.
func TestClaudeSetupRefusesOtherName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	oldPath, oldDistros := snowPath, claudeDistros
	snowPath = func() (string, error) { return "/home/u/go/bin/snow-dev", nil }
	claudeDistros = func() ([]shell.Profile, error) { return nil, nil }
	t.Cleanup(func() { snowPath, claudeDistros = oldPath, oldDistros })
	old := stdin
	stdin = strings.NewReader("y\n")
	t.Cleanup(func() { stdin = old })
	var out, errOut bytes.Buffer
	if code := run([]string{"claude", "setup"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "own name") {
		t.Fatalf("code %d, stderr %q; want a refusal", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); err == nil {
		t.Fatal("setup wrote the settings")
	}
}
