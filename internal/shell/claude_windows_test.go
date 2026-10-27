package shell

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestClaudeCmdInAwkwardFolder runs a claude.cmd in a folder whose name
// has spaces and parentheses, as "Program Files (x86)" does, through the
// command line nativeClaude builds.
func TestClaudeCmdInAwkwardFolder(t *testing.T) {
	cmdExe, err := exec.LookPath("cmd.exe")
	if err != nil {
		t.Skip(err)
	}
	dir := filepath.Join(t.TempDir(), "a b (x86)")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "claude.cmd")
	if err := os.WriteFile(script, []byte("@echo snow-claude-ok\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sys := system{goos: "windows", lookPath: func(name string) (string, error) {
		if name == "claude.cmd" {
			return script, nil
		}
		if name == "cmd.exe" {
			return cmdExe, nil
		}
		return "", exec.ErrNotFound
	}}
	p, ok := nativeClaude(sys)
	if !ok {
		t.Fatal("claude.cmd not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var out bytes.Buffer
	l := p.Launch(Hooks{}, dir)
	if code, err := l.Run(ctx, &out); err != nil || code != 0 || !strings.Contains(out.String(), "snow-claude-ok") {
		t.Fatalf("Run() = %d, %v, output %q", code, err, out.String())
	}
}
