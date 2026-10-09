package server

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/transport"
	"github.com/emmyfong/snow/pkg/api"
)

func shortSocket(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "snow")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "run", "default.sock")
}

func runAsync(ctx context.Context, t *testing.T, opts Options, path string) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, opts, path) }()
	// Wait until the socket answers.
	eventually(t, "server to listen", func() bool {
		c, err := transport.Dial(path)
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	})
	return done
}

// briefSpec runs a program that exits after about a second.
func briefSpec() (termSpec, string) {
	if runtime.GOOS == "windows" {
		return termSpec{Path: "cmd.exe", Args: []string{"/c", "ping -n 2 127.0.0.1 >nul"}}, "brief"
	}
	return termSpec{Path: "/bin/sh", Args: []string{"-c", "sleep 1"}}, "brief"
}

func TestRunExitsAfterLastSession(t *testing.T) {
	path := shortSocket(t)
	opts := testOptions()
	opts.Spec = briefSpec
	done := runAsync(context.Background(), t, opts, path)

	raw, err := transport.Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = raw.Close() }()
	c := api.NewConn(raw)
	if _, err := api.ClientHandshake(c, api.Hello{ClientVersion: "test", Protocol: api.ProtocolVersion, Cols: 80, Rows: 24}); err != nil {
		t.Fatal(err)
	}
	if err := c.Send(&api.Attach{Session: "only", Create: true}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("Run kept going after its last session ended")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("socket file left behind: %v", err)
	}
}

func TestRunIdleExit(t *testing.T) {
	opts := testOptions()
	opts.IdleExit = 200 * time.Millisecond
	done := runAsync(context.Background(), t, opts, shortSocket(t))
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run waited forever with no session")
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := runAsync(ctx, t, testOptions(), shortSocket(t))
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run ignored cancellation")
	}
}

func TestRunRefusesSecondServer(t *testing.T) {
	path := shortSocket(t)
	runAsync(context.Background(), t, testOptions(), path)
	if err := Run(context.Background(), testOptions(), path); err == nil {
		t.Fatal("a second Run on the same socket succeeded")
	}
}
