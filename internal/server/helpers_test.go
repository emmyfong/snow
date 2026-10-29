package server

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/term"
)

// testOptions runs panes with a plain system shell: /bin/sh or cmd.exe.
func testOptions() Options {
	return Options{
		Version: "test",
		Spec: func(_, dir string) (term.Spec, string, error) {
			if runtime.GOOS == "windows" {
				return term.Spec{Path: "cmd.exe", Dir: dir}, "cmd", nil
			}
			return term.Spec{Path: "/bin/sh", Dir: dir}, "sh", nil
		},
	}
}

func newTestServer(t testing.TB) *Server {
	t.Helper()
	s := New(testOptions())
	t.Cleanup(s.Close)
	return s
}

// shellProfile is the profile name testOptions gives the test shell.
func shellProfile() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "sh"
}

func prompt() string {
	if runtime.GOOS == "windows" {
		return ">"
	}
	return "$ "
}

// eventually polls cond for up to fifteen seconds.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func typeLine(p *term.Pane, s string) {
	for _, r := range s {
		p.SendKey(term.Key{Code: r, Text: string(r)})
	}
	p.SendKey(term.Key{Code: term.KeyEnter})
}

func screenHas(p *term.Pane, s string) bool { return strings.Contains(p.Text(), s) }
