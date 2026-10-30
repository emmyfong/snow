package gateway

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestGatewayHelper is the program that TestGatewayStart and its siblings
// run. The test binary starts itself again, so the tests need no external
// program and run the same way on Linux, macOS, and Windows.
func TestGatewayHelper(t *testing.T) {
	args := flag.Args()
	if len(args) == 0 {
		t.Skip("helper: started by another test")
	}
	switch args[0] {
	case "serve":
		serveHelper(args[1:])
	case "mark":
		markHelper(args[1:])
	case "exit":
		fmt.Println(args[1])
		os.Exit(3)
	default:
		fmt.Fprintf(os.Stderr, "unknown helper mode %q\n", args[0])
		os.Exit(2)
	}
}

// serveHelper listens on addr, marks that Snow started it, and answers
// every request until Snow stops it.
func serveHelper(args []string) {
	addr, marker := args[0], args[1]
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(marker, []byte("started\n"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})}
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// markHelper records that Snow ran the start command.
func markHelper(args []string) {
	if err := os.WriteFile(args[0], []byte("started\n"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// TestGatewayStart: Snow starts a gateway whose address answers nothing,
// waits for it, and stops its tree on exit.
func TestGatewayStart(t *testing.T) {
	addr, marker := freeAddr(t), filepath.Join(t.TempDir(), "started")
	spec := Spec{
		Name: "omniroute", BaseURL: "http://" + addr + "/v1",
		Start: helperArgs(t, "serve", addr, marker),
	}
	m := New([]Spec{spec}, Options{Wait: 15 * time.Second, Poll: 25 * time.Millisecond, LogDir: t.TempDir()})
	m.Start()
	st := waitStatus(t, m, StateReady)
	if st.LogPath == "" {
		t.Fatal("a gateway Snow started has no log")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("Snow did not run start: %v", err)
	}
	m.Stop()
	if err := Probe(context.Background(), spec.BaseURL); err == nil {
		t.Fatal("the gateway still answers after Stop")
	}
}

// TestGatewayAlreadyRunning: Snow does not start a second copy of a gateway
// that answers, and Stop leaves the user's copy alone.
func TestGatewayAlreadyRunning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	marker := filepath.Join(t.TempDir(), "started")
	m := New([]Spec{{
		Name: "omniroute", BaseURL: srv.URL,
		Start: helperArgs(t, "mark", marker),
	}}, Options{Wait: 5 * time.Second, Poll: 25 * time.Millisecond, LogDir: t.TempDir()})
	m.Start()
	if st := waitStatus(t, m, StateReady); st.LogPath != "" {
		t.Fatalf("Snow opened a log for a gateway it did not start: %q", st.LogPath)
	}
	// The start goroutine returns quickly; give a stray run time to show.
	time.Sleep(100 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("Snow ran start for a gateway that was already answering")
	}
	m.Stop()
	resp, err := srv.Client().Get(srv.URL + "/models")
	if err != nil {
		t.Fatalf("Stop touched the gateway the user started: %v", err)
	}
	_ = resp.Body.Close()
}

// TestGatewayFails: a start command that exits leaves the server running,
// and its output and the reason are in the log.
func TestGatewayFails(t *testing.T) {
	dir := t.TempDir()
	m := New([]Spec{{
		Name: "omniroute", BaseURL: "http://" + freeAddr(t) + "/v1",
		Start: helperArgs(t, "exit", "boom"),
	}}, Options{Wait: 10 * time.Second, Poll: 25 * time.Millisecond, LogDir: dir})

	done := make(chan struct{})
	go func() {
		m.Start()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start blocked the caller")
	}

	st := waitStatus(t, m, StateFailed)
	want := "failed, see " + filepath.Join(dir, "omniroute.log")
	if st.Detail() != want {
		t.Fatalf("detail %q, want %q", st.Detail(), want)
	}
	data, err := os.ReadFile(st.LogPath)
	if err != nil {
		t.Fatalf("read the log: %v", err)
	}
	if !strings.Contains(string(data), "boom") {
		t.Fatalf("log %q, want the program's output", data)
	}
	m.Stop()
}

// TestStatusText: the picker gets a short word, and the note names the log.
func TestStatusText(t *testing.T) {
	for _, tt := range []struct {
		st           Status
		word, detail string
	}{
		{Status{State: StateReady}, "ready", "ready"},
		{Status{State: StateStarting}, "starting", "starting"},
		{Status{State: StateFailed, LogPath: "/home/u/.snow/logs/gw.log"}, "failed", "failed, see /home/u/.snow/logs/gw.log"},
		{Status{State: StateFailed}, "failed", "failed"},
	} {
		if got := tt.st.Word(); got != tt.word {
			t.Errorf("%+v: Word %q, want %q", tt.st, got, tt.word)
		}
		if got := tt.st.Detail(); got != tt.detail {
			t.Errorf("%+v: Detail %q, want %q", tt.st, got, tt.detail)
		}
	}
}

// waitStatus returns m's single gateway once it reaches want.
func waitStatus(t *testing.T, m *Manager, want State) Status {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	var last Status
	for time.Now().Before(deadline) {
		all := m.Statuses()
		if len(all) != 1 {
			t.Fatalf("statuses %+v, want one gateway", all)
		}
		last = all[0]
		if last.State == want {
			return last
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("gateway is %s (%v), want %s", last.Word(), last.Err, Status{State: want}.Word())
	return last
}

// helperArgs runs this test binary as the gateway helper.
func helperArgs(t *testing.T, args ...string) []string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return append([]string{exe, "-test.run=TestGatewayHelper", "--"}, args...)
}

// freeAddr returns an address on this machine that nothing listens on.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}
