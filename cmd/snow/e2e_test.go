package main

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client"
	"github.com/emmyfong/snow/internal/server"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/internal/transport"
	"github.com/emmyfong/snow/pkg/api"
)

func testShell() (term.Spec, string) {
	if runtime.GOOS == "windows" {
		return term.Spec{Path: "cmd.exe"}, "cmd"
	}
	return term.Spec{Path: "/bin/sh"}, "sh"
}

func startServer(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "snow")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "run", "default.sock")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.Run(ctx, server.Options{Version: "e2e", Spec: testShell, IdleExit: time.Minute}, path)
	}()
	t.Cleanup(func() { cancel(); <-done })
	return path
}

// observer is a second client that watches a session's screen.
type observer struct {
	mu   sync.Mutex
	rows map[int]string
	raw  net.Conn
}

func observe(t *testing.T, path, session string) *observer {
	t.Helper()
	var raw net.Conn
	deadline := time.Now().Add(10 * time.Second)
	for {
		var err error
		if raw, err = transport.Dial(path); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("dial: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	conn := api.NewConn(raw)
	if _, err := api.ClientHandshake(conn, api.Hello{ClientVersion: "e2e", Protocol: api.ProtocolVersion, Cols: 200, Rows: 24}); err != nil {
		t.Fatal(err)
	}
	if err := conn.Send(&api.Attach{Session: session}); err != nil {
		t.Fatal(err)
	}
	o := &observer{rows: map[int]string{}, raw: raw}
	go func() {
		for {
			m, err := conn.Receive()
			if err != nil {
				return
			}
			// The client under test may not have created the session yet.
			if e, ok := m.(*api.Error); ok && e.Code == api.CodeNoSession {
				time.Sleep(20 * time.Millisecond)
				_ = conn.Send(&api.Attach{Session: session})
				continue
			}
			if u, ok := m.(*api.PaneUpdate); ok {
				o.mu.Lock()
				for _, l := range u.Lines {
					o.rows[l.Row] = strings.TrimRight(ansi.Strip(l.Text), " ")
				}
				o.mu.Unlock()
			}
		}
	}()
	t.Cleanup(func() { _ = raw.Close() })
	return o
}

func (o *observer) screen() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	var b strings.Builder
	for i := range 100 {
		if r, ok := o.rows[i]; ok {
			b.WriteString(r + "\n")
		}
	}
	return b.String()
}

func (o *observer) wait(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(o.screen(), want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("screen never showed %q:\n%s", want, o.screen())
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
	watch.wait(t, prompt)

	if _, err := io.WriteString(keys, "echo e2emark\r"); err != nil {
		t.Fatal(err)
	}
	watch.wait(t, "\ne2emark\n")

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
	watch.wait(t, "\nagain\n")
	if !strings.Contains(watch.screen(), "\ne2emark\n") {
		t.Fatalf("reattached screen lost earlier output:\n%s", watch.screen())
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
		time.Sleep(300 * time.Millisecond)
		if _, err := io.WriteString(keys, ctrlB+"d"); err != nil {
			t.Fatal(err)
		}
		if res := waitResult(t, done); res.Session != want {
			t.Fatalf("new session %q, want %q", res.Session, want)
		}
	}
}
