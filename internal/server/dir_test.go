package server

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

// TestNewSessionStartsInClientFolder: a session starts in the folder the
// client ran snow from, not in the folder that started the server.
func TestNewSessionStartsInClientFolder(t *testing.T) {
	dir, err := os.MkdirTemp("", "snowdir")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	s := newTestServer(t)
	c := connect(t, s, 200, 24)
	c.Send(&api.Attach{Session: "here", Create: true, Dir: dir})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	show := "pwd\r"
	if runtime.GOOS == "windows" {
		show = "cd\r"
	}
	c.Send(&api.Input{Pane: 1, Paste: show})
	c.WaitScreen(filepath.Base(dir) + "\n")
}

// TestMissingFolderFallsBack: a folder that does not exist on the server
// does not stop the session from starting.
func TestMissingFolderFallsBack(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "lost", Create: true, Dir: filepath.Join(os.TempDir(), "snow-no-such-folder")})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
}

func TestMissingFolderNotice(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	s := newTestServer(t)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "lost", Create: true, Dir: missing})
	e := nextError(t, c)
	if e.Code != api.CodeNotice || !strings.Contains(e.Message, missing) || !strings.Contains(e.Message, "home folder") {
		t.Fatalf("fallback notice %+v", e)
	}
	c.WaitScreen(strings.TrimRight(prompt(), " "))
}

func TestUsableDirReport(t *testing.T) {
	dir := t.TempDir()
	if used, fellBack := usableDirReport(dir); used != dir || fellBack {
		t.Fatalf("usable directory returned %q, fellBack %t", used, fellBack)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	missing := filepath.Join(t.TempDir(), "missing")
	if used, fellBack := usableDirReport(missing); used != home || !fellBack {
		t.Fatalf("missing directory returned %q, fellBack %t", used, fellBack)
	}
	if used, fellBack := usableDirReport(""); used != home || !fellBack {
		t.Fatalf("empty directory returned %q, fellBack %t", used, fellBack)
	}
}

func TestSplitMissingFolderNotice(t *testing.T) {
	s := newTestServer(t)
	c, first := attached(t, s, 80, 24)
	missing := filepath.Join(t.TempDir(), "missing")
	c.Send(&api.Command{Op: api.OpSplit, Target: first, Args: map[string]string{
		api.ArgDir: api.DirRight, api.ArgFolder: missing,
	}})
	e := nextError(t, c)
	if e.Code != api.CodeNotice || !strings.Contains(e.Message, missing) || !strings.Contains(e.Message, "home folder") {
		t.Fatalf("fallback notice %+v", e)
	}
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	if len(shownPanes(l)) != 2 {
		t.Fatalf("split did not start after fallback: %+v", l)
	}
}

// TestPaneEnv: a pane's shell sees SNOW with the socket path and the pane
// ID, so a nested snow can tell it runs inside a session, and SNOW_PANE and
// SNOW_SOCKET, so snow hook can tell the server which pane it runs in.
func TestPaneEnv(t *testing.T) {
	opts := testOptions()
	opts.Socket = "/snow-test/default.sock"
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 200, 24)
	c.Send(&api.Attach{Session: "env", Create: true})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	show := "echo [$SNOW] [$SNOW_PANE] [$SNOW_SOCKET]\r"
	if runtime.GOOS == "windows" {
		show = "echo [%SNOW%] [%SNOW_PANE%] [%SNOW_SOCKET%]\r"
	}
	c.Send(&api.Input{Pane: 1, Paste: show})
	c.WaitScreen("[/snow-test/default.sock,1] [1] [/snow-test/default.sock]\n")
}
