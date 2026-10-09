//go:build !windows

package term

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUnixShellEcho(t *testing.T) {
	p := startReal(t, Spec{Path: "/bin/sh"})
	waitForPrompt(t, p, "$ ")
	typeLine(p, "echo snowterm")
	waitForCount(t, p, "snowterm", 2)
}

func TestUnixResize(t *testing.T) {
	p := startReal(t, Spec{Path: "/bin/sh"})
	waitForPrompt(t, p, "$ ")
	if err := p.Resize(120, 40); err != nil {
		t.Fatal(err)
	}
	typeLine(p, "stty size")
	waitForCount(t, p, "40 120", 1)
}

func TestUnixExitDetected(t *testing.T) {
	p := startReal(t, Spec{Path: "/bin/sh"})
	waitForPrompt(t, p, "$ ")
	typeLine(p, "exit")
	waitDone(t, p, "exit")
}

func TestUnixCloseEndsLongRunningProgram(t *testing.T) {
	p := startReal(t, Spec{Path: "/bin/sh", Args: []string{"-c", "sleep 600"}})
	start := time.Now()
	if err := p.Close(); err != nil {
		t.Logf("close: %v", err)
	}
	waitDone(t, p, "close")
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("close took %v", d)
	}
}

func TestUnixDirAndTerm(t *testing.T) {
	// macOS links /tmp to /private/tmp, and pwd prints the resolved path.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := startReal(t, Spec{Path: "/bin/sh", Dir: dir})
	waitForPrompt(t, p, "$ ")
	widen(t, p) // macOS temp paths are long
	typeLine(p, `echo "dir=$(pwd) term=$TERM"`)
	waitForCount(t, p, "dir="+dir+" term=xterm-256color", 1)
}

func TestUnixControllingTerminal(t *testing.T) {
	p := startReal(t, Spec{Path: "/bin/sh"})
	waitForPrompt(t, p, "$ ")
	if strings.Contains(p.Text(), "job control turned off") {
		t.Fatalf("shell has no controlling terminal:\n%s", p.Text())
	}
}
