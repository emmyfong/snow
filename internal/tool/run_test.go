package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/shell"
)

// withShell gives env this OS's plain shell: cmd on Windows, sh elsewhere.
func withShell(env Env) Env {
	env.Shell = shell.Profile{Kind: shell.KindOther, Path: "/bin/sh"}
	if runtime.GOOS == "windows" {
		env.Shell = shell.Profile{Kind: shell.KindCmd, Path: filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")}
	}
	return env
}

func TestRunCapturesOutput(t *testing.T) {
	env := withShell(tree(t, map[string]string{"marker.txt": "here\n"}))
	cmd := `cat marker.txt && echo oops 1>&2 && exit 4`
	if runtime.GOOS == "windows" {
		cmd = `type marker.txt && echo oops 1>&2 && exit 4`
	}
	in, _ := json.Marshal(map[string]string{"command": cmd})
	got := run(t, env, "run", string(in))
	if got.IsError || strings.Join(strings.Fields(got.Content), " ") != "here oops [exit code 4]" {
		t.Fatalf("got %+v", got)
	}
}

func TestRunOutputCap(t *testing.T) {
	env := withShell(tree(t, nil))
	cmd := `i=0; while [ $i -lt 5000 ]; do echo line$i; i=$((i+1)); done`
	if runtime.GOOS == "windows" {
		cmd = `for /L %i in (0,1,4999) do @echo line%i`
	}
	in, _ := json.Marshal(map[string]string{"command": cmd})
	got := run(t, env, "run", string(in))
	if got.IsError || len(got.Content) > runBytes+200 {
		t.Fatalf("%d bytes, error %v", len(got.Content), got.IsError)
	}
	for _, want := range []string{"line0\n", "bytes cut]\n", "line4999\n", "[exit code 0]\n"} {
		if !strings.Contains(strings.ReplaceAll(got.Content, "\r\n", "\n"), want) {
			t.Errorf("output lacks %q", want)
		}
	}
}

// TestRunTimeout: a command past its timeout is killed and the result says
// so, with the output so far.
func TestRunTimeout(t *testing.T) {
	env := withShell(tree(t, nil))
	cmd := `echo started; sleep 30`
	if runtime.GOOS == "windows" {
		cmd = `echo started && ping -n 30 127.0.0.1 >nul`
	}
	in, _ := json.Marshal(map[string]any{"command": cmd, "timeout_seconds": 1})
	start := time.Now()
	got := run(t, env, "run", string(in))
	if !got.IsError || !strings.Contains(got.Content, "started") || !strings.Contains(got.Content, "[timed out after 1s; the command was killed]") {
		t.Fatalf("got %+v", got)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Fatalf("took %s", d)
	}
}

// TestRunCanceled: an interrupt ends the command and gives the context's
// error, not a timeout.
func TestRunCanceled(t *testing.T) {
	env := withShell(tree(t, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cmd := `sleep 30`
	if runtime.GOOS == "windows" {
		cmd = `ping -n 30 127.0.0.1 >nul`
	}
	in, _ := json.Marshal(map[string]string{"command": cmd})
	_, err := runTool{}.Run(ctx, env, in)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err %v", err)
	}
}

func TestRunNoShell(t *testing.T) {
	if got := run(t, tree(t, nil), "run", `{"command":"ls"}`); !got.IsError || got.Content != "this conversation has no shell profile" {
		t.Fatalf("got %+v", got)
	}
}

// TestRunWSL runs a command in a WSL folder through a detected distro. It
// runs only on Windows with WSL.
func TestRunWSL(t *testing.T) {
	wsl := wslProfile(t)
	env := Env{Folder: Folder{Path: "/tmp", Distro: wsl.Distro}, Shell: wsl}
	got := run(t, env, "run", `{"command":"pwd && uname -s"}`)
	if got.IsError || strings.Join(strings.Fields(got.Content), " ") != "/tmp Linux [exit code 0]" {
		t.Fatalf("got %+v", got)
	}
}

// TestReadWSL reads a file in a real distro through its Windows path. It
// runs only on Windows with WSL. The file is a plain one the test makes:
// Windows cannot follow a WSL symlink through \\wsl.localhost.
func TestReadWSL(t *testing.T) {
	wsl := wslProfile(t)
	dir := fmt.Sprintf("/tmp/snow-read-wsl-%d", os.Getpid())
	env := Env{Folder: Folder{Path: "/tmp", Distro: wsl.Distro}, Shell: wsl}
	if got := run(t, env, "run", `{"command":"mkdir -p `+dir+` && printf 'one\\ntwo\\n' > `+dir+`/a.txt"}`); got.IsError {
		t.Fatalf("make the file: %+v", got)
	}
	t.Cleanup(func() { run(t, env, "run", `{"command":"rm -rf `+dir+`"}`) })
	env.Folder.Path = dir
	if got := run(t, env, "read", `{"path":"a.txt"}`); got.IsError || got.Content != "     1\tone\n     2\ttwo\n" {
		t.Fatalf("got %+v", got)
	}
}

// wslProfile returns a detected WSL profile, or skips the test.
func wslProfile(t *testing.T) shell.Profile {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("WSL runs only on Windows")
	}
	for _, p := range shell.Detect() {
		if p.Kind == shell.KindWSL {
			return p
		}
	}
	t.Skip("no WSL distro")
	return shell.Profile{}
}

func TestHeadTail(t *testing.T) {
	h := newHeadTail(8)
	for _, s := range []string{"ab", "cdef", "ghij", "kl"} {
		_, _ = h.Write([]byte(s))
	}
	if got := h.String(); got != "abcd\n[4 bytes cut]\nijkl\n" {
		t.Fatalf("got %q", got)
	}
	if got := newHeadTail(8).String(); got != "" {
		t.Fatalf("empty: %q", got)
	}
}
