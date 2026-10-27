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
