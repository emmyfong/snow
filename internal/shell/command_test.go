package shell

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestCommandPerKind(t *testing.T) {
	tests := []struct {
		p       Profile
		args    []string
		dir     string
		cmdLine string
	}{
		{Profile{Kind: KindBash, Path: "/bin/bash"}, []string{"-c", "ls"}, "/w", ""},
		{Profile{Kind: KindZsh, Path: "/bin/zsh"}, []string{"-c", "ls"}, "/w", ""},
		{Profile{Kind: KindOther, Path: "/bin/fish"}, []string{"-c", "ls"}, "/w", ""},
		{Profile{Kind: KindPwsh, Path: "pwsh.exe", Args: []string{"-NoLogo"}}, []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "ls"}, "/w", ""},
		{Profile{Kind: KindCmd, Path: `C:\cmd.exe`}, []string{"/D", "/S", "/C", "ls"}, "/w", `"C:\cmd.exe" /D /S /C "ls"`},
		{Profile{Kind: KindWSL, Path: "wsl.exe", Args: []string{"-d", "Ubuntu"}, Distro: "Ubuntu"}, []string{"-d", "Ubuntu", "--cd", "/w", "-e", "sh", "-c", "ls"}, "", ""},
	}
	for _, tt := range tests {
		l := tt.p.Command("/w", "ls")
		if l.Path != tt.p.Path || !slices.Equal(l.Args, tt.args) || l.Dir != tt.dir || l.CmdLine != tt.cmdLine {
			t.Errorf("kind %d: %+v", tt.p.Kind, l)
		}
	}
	p := Profile{Kind: KindBash, Path: "bash", Args: []string{"--norc"}}
	if p.Command("/w", "ls"); !slices.Equal(p.Args, []string{"--norc"}) {
		t.Error("Command changed the profile's arguments")
	}
}

// local is the shell of this OS for tests: cmd on Windows, sh elsewhere.
func local(t *testing.T) Profile {
	t.Helper()
	if runtime.GOOS == "windows" {
		return Profile{Kind: KindCmd, Path: filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")}
	}
	return Profile{Kind: KindOther, Path: "/bin/sh"}
}

func TestRunCapturesOutput(t *testing.T) {
	dir := t.TempDir()
	line := "echo out && echo err 1>&2 && exit 3"
	var out bytes.Buffer
	code, err := local(t).Command(dir, line).Run(context.Background(), &out)
	if err != nil || code != 3 {
		t.Fatalf("code %d, err %v", code, err)
	}
	got := strings.Fields(out.String())
	if !slices.Equal(got, []string{"out", "err"}) {
		t.Fatalf("output %q", out.String())
	}
}

// TestRunQuotes: cmd gets the line as it is, quotes and all.
func TestRunQuotes(t *testing.T) {
	var out bytes.Buffer
	if _, err := local(t).Command(t.TempDir(), `echo "a  b"`).Run(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != `a  b` && got != `"a  b"` {
		t.Fatalf("output %q", got)
	}
}

// TestRunCancelKillsChildren: a cancel ends the shell and the child it
// started, and Run returns at once with the context's error.
func TestRunCancelKillsChildren(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	// The child writes a line a second; once it is dead, the file stops
	// growing. ping waits about a second between replies on Windows.
	line := `while true; do echo tick; sleep 0.2; done > ` + out
	if runtime.GOOS == "windows" {
		line = `ping -n 60 127.0.0.1 > ` + out
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := local(t).Command(dir, line).Run(ctx, &bytes.Buffer{})
		done <- err
	}()
	waitGrow(t, out, 0)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after the cancel")
	}
	before := size(out)
	time.Sleep(2500 * time.Millisecond)
	if after := size(out); after != before {
		t.Fatalf("the child still writes: %d bytes, then %d", before, after)
	}
}

// waitGrow waits until file is bigger than n bytes.
func waitGrow(t *testing.T, file string, n int64) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if size(file) > n {
			return
		}
	}
	t.Fatalf("%s did not grow", file)
}

func size(file string) int64 {
	info, err := os.Stat(file)
	if err != nil {
		return 0
	}
	return info.Size()
}
