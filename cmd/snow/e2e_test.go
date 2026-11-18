package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/client"
	"github.com/emmyfong/snow/internal/server"
	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/internal/transport"
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
	startServerAt(t, path)
	return path
}

func startServerAt(t *testing.T, path string) {
	startServerAtVersion(t, path, "e2e")
}

func startServerAtVersion(t *testing.T, path, version string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.Run(ctx, server.Options{Version: version, Spec: testShell, IdleExit: time.Minute}, path)
	}()
	t.Cleanup(func() { cancel(); <-done })
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
	// Separate Esc from y so the terminal input parser does not read Alt+y.
	go func() {
		_, _ = io.WriteString(keys, "\x1b")
		time.Sleep(100 * time.Millisecond)
		_, _ = io.WriteString(keys, "y")
	}()
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

func TestDetachPrintsNoDetachedText(t *testing.T) {
	p := startBinarySession(t, "e2e")
	detachViaMenu(t, p)
	p.SendKey(term.Key{Code: term.KeyEscape})
	waitTermText(t, p, "Leave Snow")
	p.SendKey(term.Key{Code: 'y', Text: "y"})
	waitBinaryExit(t, p)
	if strings.Contains(strings.ToLower(p.Text()), "detached") {
		t.Fatalf("output after detach contains detached text:\n%s", p.Text())
	}
}

func TestDetachPrintsLeaveLine(t *testing.T) {
	p := startBinarySession(t, "e2e")
	detachViaMenu(t, p)
	p.SendKey(term.Key{Code: term.KeyEscape})
	waitTermText(t, p, "Leave Snow")
	p.SendKey(term.Key{Code: 'y', Text: "y"})
	waitTermText(t, p, "Snow is still running 1 session. Run snow to return.")
	waitBinaryExit(t, p)
	if status, ok := p.ExitStatus(); !ok || status != 0 {
		t.Fatalf("exit status %d, known %v; want 0", status, ok)
	}
}

func startBinarySession(t *testing.T, session string) *term.Pane {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "snow")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	build := exec.Command("go", "build", "-ldflags", "-X main.version=e2e", "-o", exe, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build snow: %v\n%s", err, output)
	}
	env := binaryEnv(t)
	path, err := transport.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	startServerAt(t, path)
	p, err := term.Start(term.Spec{Path: exe, Args: []string{session}, Env: env}, 100, 30, term.Options{})
	if err != nil {
		t.Fatalf("start snow: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	waitTermText(t, p, shellPrompt())
	return p
}

func detachViaMenu(t *testing.T, p *term.Pane) {
	t.Helper()
	p.SendMouse(term.Mouse{X: 40, Y: 12, Button: term.ButtonRight, Action: term.ActionPress})
	p.SendMouse(term.Mouse{X: 40, Y: 12, Button: term.ButtonRight, Action: term.ActionRelease})
	waitTermText(t, p, "Detach")
	clickTermLabel(t, p, "Detach")
	waitTermText(t, p, "e2e")
	if strings.Contains(p.Text(), "e2e") {
		return
	}
	t.Fatalf("home page does not list e2e:\n%s", p.Text())
}

func waitBinaryExit(t *testing.T, p *term.Pane) {
	t.Helper()
	select {
	case <-p.Done():
	case <-time.After(15 * time.Second):
		t.Fatalf("snow did not exit; screen:\n%s", p.Text())
	}
	if status, ok := p.ExitStatus(); !ok || status != 0 {
		t.Fatalf("exit status %d, known %v; want 0", status, ok)
	}
}

func binaryEnv(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("HOME", dir)
	t.Setenv("LOCALAPPDATA", dir)
	return []string{"XDG_RUNTIME_DIR=" + dir, "HOME=" + dir, "LOCALAPPDATA=" + dir}
}

func waitTermText(t *testing.T, p *term.Pane, text string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(p.Text(), text) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%q did not appear; screen:\n%s", text, p.Text())
}

func clickTermLabel(t *testing.T, p *term.Pane, label string) {
	t.Helper()
	for y, line := range strings.Split(p.Text(), "\n") {
		if x := strings.Index(line, label); x >= 0 {
			p.SendMouse(term.Mouse{X: x + len(label)/2, Y: y, Button: term.ButtonLeft, Action: term.ActionPress})
			p.SendMouse(term.Mouse{X: x + len(label)/2, Y: y, Button: term.ButtonLeft, Action: term.ActionRelease})
			return
		}
	}
	t.Fatalf("%q not found; screen:\n%s", label, p.Text())
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

// TestShellExitShowsHome: when the last shell exits, the client shows the
// home page instead of quitting; Esc there quits, with no error.
func TestShellExitShowsHome(t *testing.T) {
	path := startServer(t)
	keys, done := attachClient(path, "brief")
	w := apitest.Dial(t, path, 80, 24)
	defer w.Close()
	waitAttached(t, w, "brief", 1)
	watch := observe(t, path, "brief")
	watch.WaitScreen(shellPrompt())
	if _, err := io.WriteString(keys, "exit\r"); err != nil {
		t.Fatal(err)
	}
	waitGone(t, w, "brief")
	select {
	case r := <-done:
		t.Fatalf("the client quit when the session ended: %+v", r)
	case <-time.After(500 * time.Millisecond):
	}
	go func() {
		_, _ = io.WriteString(keys, "\x1b")
		time.Sleep(100 * time.Millisecond)
		_, _ = io.WriteString(keys, "y")
	}()
	if res := waitResult(t, done); res.Err != nil || res.Detached {
		t.Fatalf("result %+v, want a plain quit from the home page", res)
	}
	_ = keys.Close() // unblocks a write the quit client never reads
}

// waitGone reads the session lists w gets until session is not in one.
func waitGone(t *testing.T, w *apitest.Client, session string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		m, _ := w.Next(time.Second)
		if l, ok := m.(*api.SessionList); ok && !slices.ContainsFunc(l.Sessions, func(s api.SessionInfo) bool { return s.Name == session }) {
			return
		}
	}
	t.Fatalf("session %s never ended", session)
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
