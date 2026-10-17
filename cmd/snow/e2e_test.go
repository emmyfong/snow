package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/client"
	"github.com/emmyfong/snow/internal/server"
	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

func testShell(_, dir string) (term.Spec, string, error) {
	if runtime.GOOS == "windows" {
		return term.Spec{Path: "cmd.exe", Dir: dir}, "cmd", nil
	}
	return term.Spec{Path: "/bin/sh", Dir: dir}, "sh", nil
}

func startServer(t *testing.T) string {
	t.Helper()
	path := apitest.SocketPath(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.Run(ctx, server.Options{Version: "e2e", Spec: testShell, IdleExit: time.Minute}, path)
	}()
	t.Cleanup(func() { cancel(); <-done })
	return path
}

// observe attaches a second client that watches a session's screen. It
// waits for the client under test to create the session.
func observe(t *testing.T, path, session string) *apitest.Client {
	t.Helper()
	o := apitest.Dial(t, path, 200, 24)
	o.Attach(session)
	return o
}

type runResult struct {
	res client.Result
	err error
}

// attachClient runs a real client against path with keys from the returned
// writer.
func attachClient(path, session string) (io.WriteCloser, <-chan runResult) {
	return attachClientIn(path, session, "")
}

// attachClientIn is attachClient for a client run from dir.
func attachClientIn(path, session, dir string) (io.WriteCloser, <-chan runResult) {
	in, keys := io.Pipe()
	done := make(chan runResult, 1)
	go func() {
		res, err := client.Run(client.Options{
			Version: "e2e", Socket: path, Session: session, Dir: dir,
			Start: func() error { return nil },
			Input: in, Output: io.Discard,
		})
		done <- runResult{res, err}
	}()
	return keys, done
}

func waitResult(t *testing.T, done <-chan runResult) client.Result {
	t.Helper()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("client.Run: %v", r.err)
		}
		return r.res
	case <-time.After(15 * time.Second):
		t.Fatal("client did not return")
		return client.Result{}
	}
}

const ctrlB = "\x02"

// detach presses Ctrl+b d, which leaves session and shows the home page,
// then Esc to quit it, and returns how the client ended. others is how many
// other clients watch session. Esc goes only once the server counts the
// client gone: by then the home page is up, so the Esc cannot reach the
// pane.
func detach(t *testing.T, path, session string, others int, keys io.Writer, done <-chan runResult) client.Result {
	t.Helper()
	// One watcher for the whole wait. Dialing a probe per check would let
	// the server, with no session yet, exit as each probe leaves (ADR 0008).
	w := apitest.Dial(t, path, 80, 24)
	defer w.Close()
	waitAttached(t, w, session, others+1)
	if _, err := io.WriteString(keys, ctrlB+"d"); err != nil {
		t.Fatal(err)
	}
	waitAttached(t, w, session, others)
	// The pipe blocks until the client reads; a client that has already
	// quit never does, so the write must not hold up the test.
	go func() { _, _ = io.WriteString(keys, "\x1b") }()
	return waitResult(t, done)
}

// waitAttached reads the session lists w gets, as a client that has not
// attached, until session has n clients attached.
func waitAttached(t *testing.T, w *apitest.Client, session string, n int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		m, _ := w.Next(time.Second)
		if l, ok := m.(*api.SessionList); ok {
			for _, s := range l.Sessions {
				if s.Name == session && s.Attached == n {
					return
				}
			}
		}
	}
	t.Fatalf("session %s never had %d clients attached", session, n)
}

func shellPrompt() string {
	if runtime.GOOS == "windows" {
		return ">"
	}
	return "$"
}

func TestDetachAndReattach(t *testing.T) {
	path := startServer(t)

	keys, done := attachClient(path, "e2e")
	watch := observe(t, path, "e2e")
	watch.WaitScreen(shellPrompt())

	if _, err := io.WriteString(keys, "echo e2emark\r"); err != nil {
		t.Fatal(err)
	}
	watch.WaitScreen("\ne2emark\n")

	res := detach(t, path, "e2e", 1, keys, done)
	if !res.Detached || res.Session != "e2e" {
		t.Fatalf("result %+v, want detached from e2e", res)
	}

	// The session kept running: the observer still sees it, and a new client
	// reattaches to the same screen and types into it.
	keys2, done2 := attachClient(path, "e2e")
	if _, err := io.WriteString(keys2, "echo again\r"); err != nil {
		t.Fatal(err)
	}
	watch.WaitScreen("\nagain\n")
	if !strings.Contains(watch.Screen(), "\ne2emark\n") {
		t.Fatalf("reattached screen lost earlier output:\n%s", watch.Screen())
	}
	if res := detach(t, path, "e2e", 1, keys2, done2); !res.Detached {
		t.Fatalf("second client result %+v, want detached", res)
	}
}

func TestPlainSnowCreatesNumberedSessions(t *testing.T) {
	path := startServer(t)
	for _, want := range []string{"0", "1"} {
		keys, done := attachClient(path, "")
		if res := detach(t, path, want, 0, keys, done); res.Session != want {
			t.Fatalf("new session %q, want %q", res.Session, want)
		}
	}
}

func TestSessionStartsInClientFolder(t *testing.T) {
	dir, err := os.MkdirTemp("", "snowdir")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := startServer(t)
	keys, done := attachClientIn(path, "here", dir)
	watch := observe(t, path, "here")
	watch.WaitScreen(shellPrompt())
	show := "pwd\r"
	if runtime.GOOS == "windows" {
		show = "cd\r"
	}
	if _, err := io.WriteString(keys, show); err != nil {
		t.Fatal(err)
	}
	watch.WaitScreen(filepath.Base(dir) + "\n")
	detach(t, path, "here", 1, keys, done)
}

// TestShellExitEndsClient: when the shell exits, the client reports that the
// session ended, not a lost connection.
func TestShellExitEndsClient(t *testing.T) {
	path := startServer(t)
	keys, done := attachClient(path, "brief")
	watch := observe(t, path, "brief")
	watch.WaitScreen(shellPrompt())
	if _, err := io.WriteString(keys, "exit\r"); err != nil {
		t.Fatal(err)
	}
	if res := waitResult(t, done); !res.Ended || res.Session != "brief" {
		t.Fatalf("result %+v, want session brief ended", res)
	}
}

// TestHomePageCreatesASession: plain snow opens the home page; typing a name
// and Enter creates that session and attaches to it.
func TestHomePageCreatesASession(t *testing.T) {
	path := startServer(t)
	in, keys := io.Pipe()
	done := make(chan runResult, 1)
	go func() {
		res, err := client.Run(client.Options{
			Version: "e2e", Socket: path, Home: true,
			Start: func() error { return nil },
			Input: in, Output: io.Discard,
		})
		done <- runResult{res, err}
	}()
	if _, err := io.WriteString(keys, "work\r"); err != nil {
		t.Fatal(err)
	}
	watch := observe(t, path, "work")
	watch.WaitScreen(shellPrompt())
	if res := detach(t, path, "work", 1, keys, done); !res.Detached || res.Session != "work" {
		t.Fatalf("result %+v, want detached from work", res)
	}
}

// TestRestartServerSavesAndStops: the restart reaches a real server over
// its socket; the server saves its sessions and exits, and a new server
// lists them as saved.
func TestRestartServerSavesAndStops(t *testing.T) {
	path := apitest.SocketPath(t)
	dir := t.TempDir()
	oldCtx, stopOld := context.WithCancel(context.Background())
	t.Cleanup(stopOld) // if the restart fails, the old server must still stop
	done := make(chan error, 1)
	go func() {
		done <- server.Run(oldCtx, server.Options{Version: "old", Spec: testShell, SessionsDir: dir, IdleExit: time.Minute}, path)
	}()
	o := apitest.Dial(t, path, 80, 24)
	o.Send(&api.Attach{Session: "work", Create: true})
	o.Attach("work") // waits until the session exists
	if err := client.RestartServer(path, "new"); err != nil {
		t.Fatalf("RestartServer: %v", err)
	}
	// The socket goes only after the old server saved, so the new server
	// finds the session the moment the restart returns.
	if st, err := store.Open(dir); err != nil {
		t.Fatal(err)
	} else if infos, _ := st.List(); len(infos) != 1 {
		t.Fatalf("saved %v when RestartServer returned, want work", infos)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("server: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the server kept running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	doneNew := make(chan error, 1)
	go func() {
		doneNew <- server.Run(ctx, server.Options{Version: "new", Spec: testShell, SessionsDir: dir, IdleExit: time.Minute}, path)
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-doneNew; err != nil {
			t.Errorf("new server: %v", err)
		}
	})
	c := apitest.Dial(t, path, 80, 24)
	m, _ := c.Next(5 * time.Second)
	if l, ok := m.(*api.SessionList); !ok || len(l.Sessions) != 1 || l.Sessions[0].State != api.StateSaved {
		t.Fatalf("new server lists %#v, want work as saved", m)
	}
}
