package main

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/xpty"
)

func testShell() string {
	if runtime.GOOS == "windows" {
		return "cmd.exe"
	}
	return "/bin/sh"
}

func start(t *testing.T, name string) *shell {
	t.Helper()
	s, err := startShell(name, nil, 80, 24)
	if err != nil {
		t.Fatalf("start %s: %v", name, err)
	}
	t.Cleanup(s.close)
	return s
}

// typeLine sends text and Enter through the emulator, the same path keys take
// from Bubble Tea, so the test covers key encoding as well as the PTY.
func typeLine(s *shell, text string) {
	s.emu.SendText(text)
	s.emu.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
}

// screenText reads the screen through Render, which SafeEmulator locks.
// SafeEmulator embeds Emulator, so String compiles but skips the lock and races.
func screenText(s *shell) string {
	return ansi.Strip(s.emu.Render())
}

// waitForCount polls the screen until text appears n times. The typed command
// line holds one copy, so n=2 means the shell printed it too.
func waitForCount(t *testing.T, s *shell, text string, n int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Count(screenText(s), text) >= n {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%q did not appear %d times; screen:\n%s", text, n, screenText(s))
}

func TestShellEcho(t *testing.T) {
	s := start(t, testShell())
	typeLine(s, "echo snowspike")
	waitForCount(t, s, "snowspike", 2)
}

func TestPwshEcho(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh not installed")
	}
	s := start(t, pwsh)
	typeLine(s, "Write-Output snowspike")
	waitForCount(t, s, "snowspike", 2)
}

func TestExitDetected(t *testing.T) {
	s := start(t, testShell())
	done := make(chan error, 1)
	go func() { done <- xpty.WaitProcess(context.Background(), s.cmd) }()
	typeLine(s, "exit")
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatalf("shell exit not detected; screen:\n%s", screenText(s))
	}
}

func TestResize(t *testing.T) {
	s := start(t, testShell())
	if err := s.resize(120, 40); err != nil {
		t.Fatal(err)
	}
	w, h, err := s.pty.Size()
	if err != nil {
		t.Fatal(err)
	}
	if w != 120 || h != 40 || s.emu.Width() != 120 || s.emu.Height() != 40 {
		t.Fatalf("pty %dx%d, emulator %dx%d; want 120x40", w, h, s.emu.Width(), s.emu.Height())
	}
}
