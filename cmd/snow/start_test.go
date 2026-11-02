package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/client"
	"github.com/emmyfong/snow/internal/transport"
	"github.com/emmyfong/snow/pkg/api"
)

// asSnow makes the test binary act as the snow binary, so tests can start a
// real detached server with client.StartServer.
const asSnow = "SNOW_TEST_AS_SNOW"

func TestMain(m *testing.M) {
	if role := os.Getenv(probeEnv); role != "" {
		os.Exit(runProbe(role))
	}
	if os.Getenv(asSnow) == "1" {
		os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
	}
	// The tests may run inside a snow pane, as agents building Snow do. The
	// nested-session guard must not see that pane; tests that need SNOW set
	// it themselves.
	_ = os.Unsetenv(transport.EnvName)
	os.Exit(m.Run())
}

// TestRealDetachedServer starts the server the way snow does, as a detached
// process, then talks to it and ends it by closing its only pane, which ends
// its only session.
func TestRealDetachedServer(t *testing.T) {
	dir, err := os.MkdirTemp("", "snow")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	// The server finds its socket folder from these; the child inherits them.
	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("LOCALAPPDATA", dir)
	// A new session opens a chat, which saves its conversation under the
	// home folder; the child must not write into the real one.
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv(asSnow, "1")
	path, err := transport.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := client.StartServer(exe, "server"); err != nil {
		t.Fatal(err)
	}

	c := apitest.Dial(t, path, 80, 24)
	if m, err := c.Next(apitest.Wait); err != nil {
		t.Fatalf("no session list on connect: %v", err)
	} else if _, ok := m.(*api.SessionList); !ok {
		t.Fatalf("first message %#v, want the session list", m)
	}
	if m, err := c.Next(apitest.Wait); err != nil {
		t.Fatalf("no agent list on connect: %v", err)
	} else if _, ok := m.(*api.AgentList); !ok {
		t.Fatalf("second message %#v, want the agent list", m)
	}
	c.Send(&api.Attach{Session: "real", Create: true, Dir: dir})
	m, err := c.Next(apitest.Wait)
	if l, ok := m.(*api.Layout); !ok || l.Session != "real" {
		t.Fatalf("first message after Attach %#v (%v), want the Layout of session real", m, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "server.log")); err != nil {
		t.Fatalf("server log: %v", err)
	}

	// Closing the only pane ends the only session. Then the client leaves,
	// and with no session and no client the server exits.
	deadline := time.Now().Add(apitest.Wait)
	c.Send(&api.Command{Op: api.OpClose, Target: 1})
	ended := false
	for !ended && time.Now().Before(deadline) {
		m, _ := c.Next(50 * time.Millisecond)
		if e, ok := m.(*api.Error); ok && e.Code == api.CodeSessionEnded {
			ended = true
		}
	}
	if !ended {
		t.Fatal("the session did not end after its pane closed")
	}
	c.Close()
	for time.Now().Before(deadline.Add(apitest.Wait)) {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the server did not exit after its last session and client left")
}
