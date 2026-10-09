package term

import (
	"os/exec"
	"testing"
	"time"
)

func TestWindowsCmdEcho(t *testing.T) {
	p := startReal(t, Spec{Path: "cmd.exe"})
	waitForPrompt(t, p, ">")
	widen(t, p)
	typeLine(p, "echo snowterm")
	waitForCount(t, p, "snowterm", 2)
}

func TestWindowsPwshEcho(t *testing.T) {
	path := ""
	for _, name := range []string{"pwsh.exe", "powershell.exe"} {
		if found, err := exec.LookPath(name); err == nil {
			path = found
			break
		}
	}
	if path == "" {
		t.Skip("no PowerShell installed")
	}
	p := startReal(t, Spec{Path: path, Args: []string{"-NoLogo", "-NoProfile"}})
	waitForPrompt(t, p, "PS ")
	widen(t, p)
	typeLine(p, "Write-Output snowterm")
	waitForCount(t, p, "snowterm", 2)
}

func TestWindowsExitDetected(t *testing.T) {
	p := startReal(t, Spec{Path: "cmd.exe"})
	waitForPrompt(t, p, ">")
	typeLine(p, "exit")
	waitDone(t, p, "exit")
}

func TestWindowsCloseEndsLongRunningProgram(t *testing.T) {
	p := startReal(t, Spec{Path: "cmd.exe", Args: []string{"/c", "ping -n 600 127.0.0.1"}})
	start := time.Now()
	if err := p.Close(); err != nil {
		t.Logf("close: %v", err)
	}
	waitDone(t, p, "close")
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("close took %v", d)
	}
}
