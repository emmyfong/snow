//go:build !windows

package shell

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/term"
)

// These tests run the real hook in a real shell on a PTY. They import term,
// which only tests may do: shell itself stays a leaf package.

func startHooked(t *testing.T, p Profile, dir string) *term.Pane {
	t.Helper()
	h, err := WriteHooks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l := p.Launch(h, dir)
	pane, err := term.Start(term.Spec{Path: l.Path, Args: l.Args, Env: l.Env, Dir: l.Dir}, 80, 24, term.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pane.Close() })
	return pane
}

func waitFolder(t *testing.T, p *term.Pane, want string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if p.Folder() == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("Folder() = %q, want %q; screen:\n%s", p.Folder(), want, p.Text())
}

func typeLine(p *term.Pane, s string) {
	for _, r := range s {
		p.SendKey(term.Key{Code: r, Text: string(r)})
	}
	p.SendKey(term.Key{Code: term.KeyEnter})
}

func testHookReportsFolder(t *testing.T, shellName string, kind Kind) {
	path, err := exec.LookPath(shellName)
	if err != nil {
		t.Skipf("%s not installed", shellName)
	}
	t.Setenv("HOME", t.TempDir()) // no user startup files: test the hook alone
	p := startHooked(t, Profile{Kind: kind, Path: path}, "/")
	waitFolder(t, p, "/") // the first prompt reports the start folder
	typeLine(p, "cd /tmp")
	waitFolder(t, p, "/tmp")
	if strings.Contains(p.Text(), "command not found") {
		t.Fatalf("hook error on screen:\n%s", p.Text())
	}
}

func TestBashHookReportsFolder(t *testing.T) { testHookReportsFolder(t, "bash", KindBash) }
func TestZshHookReportsFolder(t *testing.T)  { testHookReportsFolder(t, "zsh", KindZsh) }
