package main

import (
	"context"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/client"
	"github.com/emmyfong/snow/internal/server"
	"github.com/emmyfong/snow/internal/term"
)

func testShell() (term.Spec, string) {
	if runtime.GOOS == "windows" {
		return term.Spec{Path: "cmd.exe"}, "cmd"
	}
	return term.Spec{Path: "/bin/sh"}, "sh"
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
	in, keys := io.Pipe()
	done := make(chan runResult, 1)
	go func() {
		res, err := client.Run(client.Options{
			Version: "e2e", Socket: path, Session: session,
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

func TestDetachAndReattach(t *testing.T) {
	path := startServer(t)

	keys, done := attachClient(path, "e2e")
	watch := observe(t, path, "e2e")
	prompt := "$"
	if runtime.GOOS == "windows" {
		prompt = ">"
	}
	watch.WaitScreen(prompt)

	if _, err := io.WriteString(keys, "echo e2emark\r"); err != nil {
		t.Fatal(err)
	}
	watch.WaitScreen("\ne2emark\n")

	if _, err := io.WriteString(keys, ctrlB+"d"); err != nil {
		t.Fatal(err)
	}
	res := waitResult(t, done)
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
	if _, err := io.WriteString(keys2, ctrlB+"d"); err != nil {
		t.Fatal(err)
	}
	if res := waitResult(t, done2); !res.Detached {
		t.Fatalf("second client result %+v, want detached", res)
	}
}

func TestPlainSnowCreatesNumberedSessions(t *testing.T) {
	path := startServer(t)
	for _, want := range []string{"0", "1"} {
		keys, done := attachClient(path, "")
		if _, err := io.WriteString(keys, ctrlB+"d"); err != nil {
			t.Fatal(err)
		}
		if res := waitResult(t, done); res.Session != want {
			t.Fatalf("new session %q, want %q", res.Session, want)
		}
	}
}
