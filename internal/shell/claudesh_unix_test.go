//go:build !windows

package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestClaudeScriptPassesArguments: the WSL launcher hands its arguments to
// claude unchanged, a blank inside one included. It runs the script with
// the user's real login shell and a fake claude first on PATH.
func TestClaudeScriptPassesArguments(t *testing.T) {
	h, err := WriteHooks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bin, home := t.TempDir(), t.TempDir()
	out := filepath.Join(t.TempDir(), "args")
	fake := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done > \"$SNOW_TEST_OUT\"\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(fake), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", filepath.Join(h.Dir, "claude.sh"), "--permission-mode", "plan", "--append-system-prompt-file", "/tmp/a role.md")
	cmd.Env = append(os.Environ(), "HOME="+home, "PATH="+bin+":"+os.Getenv("PATH"), "SNOW_TEST_OUT="+out)
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("the login shell could not run the script here: %v\n%s", err, msg)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Skipf("the login shell did not find the fake claude (its profile may reset PATH): %v", err)
	}
	got := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	want := []string{"--permission-mode", "plan", "--append-system-prompt-file", "/tmp/a role.md"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("claude got %q, want %q", got, want)
	}
}
