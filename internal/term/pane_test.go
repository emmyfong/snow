package term

import (
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"
)

func newTestPane(t *testing.T) (*Pane, *fakePTY) {
	t.Helper()
	f := newFakePTY()
	p := newPane(f, 80, 24, Options{})
	t.Cleanup(func() { _ = p.Close() })
	return p, f
}

// eventually polls cond for up to two seconds.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestPaneRendersShellOutput(t *testing.T) {
	p, f := newTestPane(t)
	f.shellWrites("hello from the shell")
	eventually(t, "output on screen", func() bool {
		return strings.Contains(p.Text(), "hello from the shell")
	})
}

func TestPaneSendKeyEncodesForTerminalMode(t *testing.T) {
	p, f := newTestPane(t)

	p.SendKey(Key{Code: KeyUp})
	eventually(t, "normal-mode up arrow", func() bool { return f.sent() == "\x1b[A" })

	// vim and other full-screen programs turn on application cursor keys
	// (DECCKM). Up must then be encoded differently.
	// Output is parsed in order, so once the marker shows, the mode is set.
	f.shellWrites("\x1b[?1hready")
	eventually(t, "mode switch applied", func() bool { return strings.Contains(p.Text(), "ready") })
	p.SendKey(Key{Code: KeyUp})
	eventually(t, "application-mode up arrow", func() bool { return f.sent() == "\x1b[A\x1bOA" })
}

func TestPaneSendKeyText(t *testing.T) {
	p, f := newTestPane(t)
	p.SendKey(Key{Code: 'a', Text: "a"})
	p.SendKey(Key{Code: KeyEnter})
	eventually(t, "typed text", func() bool { return f.sent() == "a\r" })
}

func TestPaneResize(t *testing.T) {
	p, f := newTestPane(t)
	if err := p.Resize(120, 40); err != nil {
		t.Fatal(err)
	}
	cols, rows := p.Size()
	if cols != 120 || rows != 40 || f.cols != 120 || f.rows != 40 {
		t.Fatalf("pane %dx%d, pty %dx%d; want 120x40", cols, rows, f.cols, f.rows)
	}
}

func TestPaneDoneWhenShellExits(t *testing.T) {
	p, f := newTestPane(t)
	if _, ok := p.ExitStatus(); ok {
		t.Fatal("a running pane reported an exit status")
	}
	f.exit()
	select {
	case <-p.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Done not closed after the shell exited")
	}
	if code, ok := p.ExitStatus(); !ok || code != 0 {
		t.Fatalf("ExitStatus = %d, %v; want 0, true", code, ok)
	}
}

func TestPaneCloseStopsGoroutines(t *testing.T) {
	before := runtime.NumGoroutine()
	f := newFakePTY()
	p := newPane(f, 80, 24, Options{})
	f.shellWrites("some output")
	closed := make(chan error, 1)
	go func() { closed <- p.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close hung: a copy goroutine did not stop")
	}
	eventually(t, "goroutines to stop", func() bool { return runtime.NumGoroutine() <= before })
	if err := p.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// stuckReadPTY is a PTY whose Read never returns, even after Close, as a
// ConPTY read can when the console host does not exit.
type stuckReadPTY struct {
	*fakePTY
	block chan struct{}
}

func (s *stuckReadPTY) Read([]byte) (int, error) {
	<-s.block
	return 0, io.EOF
}

// TestCloseGivesUpOnStuckRead: a read that never returns must not hang
// Close, and so the server's shutdown.
func TestCloseGivesUpOnStuckRead(t *testing.T) {
	defer func(d time.Duration) { closeWait = d }(closeWait)
	closeWait = 100 * time.Millisecond
	pty := &stuckReadPTY{fakePTY: newFakePTY(), block: make(chan struct{})}
	defer close(pty.block)
	p := newPane(pty, 80, 24, Options{})
	done := make(chan error, 1)
	go func() { done <- p.Close() }()
	select {
	case err := <-done:
		if !errors.Is(err, ErrCloseTimeout) {
			t.Fatalf("Close = %v, want ErrCloseTimeout", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close hung on a read that never returns")
	}
}

// stuckClosePTY is a PTY whose Close never returns, as a ConPTY close can
// when CancelIoEx blocks on the output pipe (#356).
type stuckClosePTY struct {
	*fakePTY
	block chan struct{}
}

func (s *stuckClosePTY) Close() error {
	<-s.block
	return s.fakePTY.Close()
}

// TestCloseGivesUpOnStuckPTYClose: a PTY close that never returns must not
// hang Close, and so the server's shutdown.
func TestCloseGivesUpOnStuckPTYClose(t *testing.T) {
	defer func(d time.Duration) { closeWait = d }(closeWait)
	closeWait = 100 * time.Millisecond
	pty := &stuckClosePTY{fakePTY: newFakePTY(), block: make(chan struct{})}
	defer close(pty.block)
	p := newPane(pty, 80, 24, Options{})
	done := make(chan error, 1)
	go func() { done <- p.Close() }()
	select {
	case err := <-done:
		if !errors.Is(err, ErrCloseTimeout) {
			t.Fatalf("Close = %v, want ErrCloseTimeout", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close hung on a PTY close that never returns")
	}
}

// slowCloseStuckReadPTY is a PTY whose Close takes delay and whose Read
// never returns.
type slowCloseStuckReadPTY struct {
	*stuckReadPTY
	delay time.Duration
}

func (s *slowCloseStuckReadPTY) Close() error {
	time.Sleep(s.delay)
	return s.fakePTY.Close()
}

// TestCloseWaitsOnceInAll: a slow PTY close and a stuck read share one
// closeWait, and a second Close returns the first one's error at once.
func TestCloseWaitsOnceInAll(t *testing.T) {
	defer func(d time.Duration) { closeWait = d }(closeWait)
	closeWait = 400 * time.Millisecond
	pty := &slowCloseStuckReadPTY{
		stuckReadPTY: &stuckReadPTY{fakePTY: newFakePTY(), block: make(chan struct{})},
		delay:        300 * time.Millisecond,
	}
	defer close(pty.block)
	p := newPane(pty, 80, 24, Options{})
	start := time.Now()
	err := p.Close()
	elapsed := time.Since(start)
	if !errors.Is(err, ErrCloseTimeout) {
		t.Fatalf("Close = %v, want ErrCloseTimeout", err)
	}
	// Two separate limits would take about 700 ms.
	if !raceDetector && elapsed > 600*time.Millisecond {
		t.Fatalf("Close took %v, want about one closeWait (%v)", elapsed, closeWait)
	}
	start = time.Now()
	if again := p.Close(); !errors.Is(again, err) {
		t.Fatalf("second Close = %v, want %v", again, err)
	}
	if d := time.Since(start); d > 50*time.Millisecond {
		t.Fatalf("second Close took %v, want at once", d)
	}
}
