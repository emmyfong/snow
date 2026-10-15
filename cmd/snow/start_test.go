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
	os.Exit(m.Run())
}

// TestRealDetachedServer starts the server the way snow does, as a detached
// process, then talks to it and ends it by ending its only session.
func TestRealDetachedServer(t *testing.T) {
	dir, err := os.MkdirTemp("", "snow")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	// The server finds its socket folder from these; the child inherits them.
	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("LOCALAPPDATA", dir)
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
	c.Send(&api.Attach{Session: "real", Create: true, Dir: dir})
	m, err := c.Next(apitest.Wait)
	if l, ok := m.(*api.Layout); !ok || l.Session != "real" {
		t.Fatalf("first message %#v (%v), want the Layout of session real", m, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "server.log")); err != nil {
		t.Fatalf("server log: %v", err)
	}

	// Typing exit ends the shell, the only session, and so the server. The
	// user's real shell runs here, with an unknown prompt, and shells discard
	// input while they start, so type it again until the server is gone.
	deadline := time.Now().Add(apitest.Wait)
	for time.Now().Before(deadline) {
		for _, r := range "exit" {
			c.Send(&api.Input{Pane: 1, Key: &api.Key{Code: string(r), Text: string(r)}})
		}
		c.Send(&api.Input{Pane: 1, Key: &api.Key{Code: api.KeyEnter}})
		for range 20 {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	t.Fatal("the server did not exit after its last session ended")
}
