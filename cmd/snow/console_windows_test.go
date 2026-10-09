package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/client"
	"golang.org/x/sys/windows"
)

// probeEnv selects a probe role for the test binary: "spawn" runs a console
// child like the server runs wsl.exe; "report" writes whether its console
// window is visible to the file in probeOut.
const (
	probeEnv = "SNOW_TEST_PROBE"
	probeOut = "SNOW_TEST_PROBE_OUT"
)

func runProbe(role string) int {
	switch role {
	case "spawn":
		cmd := exec.Command(os.Args[0])
		cmd.Env = append(os.Environ(), probeEnv+"=report")
		if err := cmd.Run(); err != nil {
			return 1
		}
		return 0
	case "report":
		user32 := windows.NewLazySystemDLL("user32.dll")
		kernel32 := windows.NewLazySystemDLL("kernel32.dll")
		hwnd, _, _ := kernel32.NewProc("GetConsoleWindow").Call()
		visible, _, _ := user32.NewProc("IsWindowVisible").Call(hwnd)
		state := "hidden"
		if visible != 0 {
			state = "visible"
		}
		if err := os.WriteFile(os.Getenv(probeOut), []byte(state), 0o600); err != nil {
			return 1
		}
		return 0
	}
	return 2
}

// TestServerChildrenHaveNoVisibleConsole: console programs the detached
// server runs, such as wsl.exe for shell detection, must not flash a window.
func TestServerChildrenHaveNoVisibleConsole(t *testing.T) {
	out := filepath.Join(t.TempDir(), "console.txt")
	t.Setenv(probeEnv, "spawn")
	t.Setenv(probeOut, out)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := client.StartServer(exe); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		b, err := os.ReadFile(out)
		if err == nil && len(b) > 0 {
			if got := strings.TrimSpace(string(b)); got != "hidden" {
				t.Fatalf("a console program run by the server has a %s window", got)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal(fmt.Errorf("probe never reported: %w", err))
		}
		time.Sleep(50 * time.Millisecond)
	}
}
