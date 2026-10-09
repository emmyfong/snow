package shell

import (
	"os/exec"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/term"
)

func startHooked(t *testing.T, p Profile, dir string) *term.Pane {
	t.Helper()
	h, err := WriteHooks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l := p.Launch(h, dir)
	pane, err := term.Start(term.Spec{Path: l.Path, Args: l.Args, Env: l.Env, Dir: l.Dir}, 120, 30, term.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pane.Close() })
	return pane
}

func waitFolder(t *testing.T, p *term.Pane, want string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
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

func TestCmdHookReportsFolder(t *testing.T) {
	path, err := exec.LookPath("cmd.exe")
	if err != nil {
		t.Skip("cmd.exe not found")
	}
	p := startHooked(t, Profile{Kind: KindCmd, Path: path}, `C:\`)
	waitFolder(t, p, `C:\`)
	typeLine(p, `cd C:\Windows`)
	waitFolder(t, p, `C:\Windows`)
}

func TestPowerShellHookReportsFolder(t *testing.T) {
	var path string
	for _, name := range []string{"pwsh.exe", "powershell.exe"} {
		if found, err := exec.LookPath(name); err == nil {
			path = found
			break
		}
	}
	if path == "" {
		t.Skip("no PowerShell installed")
	}
	p := startHooked(t, Profile{Kind: KindPwsh, Path: path, Args: []string{"-NoLogo", "-NoProfile"}}, `C:\`)
	waitFolder(t, p, `C:\`)
	typeLine(p, `Set-Location C:\Windows`)
	waitFolder(t, p, `C:\Windows`)
}

// TestWSLHookReportsFolder runs only on machines with a WSL distribution.
func TestWSLHookReportsFolder(t *testing.T) {
	var wsl Profile
	for _, p := range Detect() {
		if p.Kind == KindWSL {
			wsl = p
			break
		}
	}
	if wsl.Path == "" {
		t.Skip("no WSL distribution installed")
	}
	p := startHooked(t, wsl, "/tmp")
	waitFolder(t, p, "/tmp")
	typeLine(p, "cd /var")
	waitFolder(t, p, "/var")
	t.Logf("distro %s: folder tracking works", wsl.Distro)
}
