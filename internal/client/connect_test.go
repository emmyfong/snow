package client

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/transport"
)

func socketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "snow")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "run", "default.sock")
}

// listenLater starts listening at path after delay, like a server that takes
// a moment to come up, and accepts connections until the test ends.
func listenLater(t *testing.T, path string, delay time.Duration) func() error {
	return func() error {
		go func() {
			time.Sleep(delay)
			l, err := transport.Listen(path)
			if err != nil {
				t.Error(err)
				return
			}
			t.Cleanup(func() { _ = l.Close() })
			for {
				c, err := l.Accept()
				if err != nil {
					return
				}
				_ = c.Close()
			}
		}()
		return nil
	}
}

func TestConnectStartsServerWhenMissing(t *testing.T) {
	path := socketPath(t)
	started := 0
	start := listenLater(t, path, 300*time.Millisecond)
	c, err := Connect(path, func() error { started++; return start() })
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	_ = c.Close()
	if started != 1 {
		t.Fatalf("start called %d times, want 1", started)
	}
}

func TestConnectUsesRunningServer(t *testing.T) {
	path := socketPath(t)
	l, err := transport.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	c, err := Connect(path, func() error { t.Fatal("started a server while one was running"); return nil })
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
}

func TestConnectGivesUp(t *testing.T) {
	path := socketPath(t)
	start := time.Now()
	_, err := Connect(path, func() error { return nil }) // "starts" but nothing listens
	if !errors.Is(err, ErrServerDidNotStart) {
		t.Fatalf("Connect error = %v, want ErrServerDidNotStart", err)
	}
	if d := time.Since(start); d > 6*time.Second {
		t.Fatalf("gave up after %v", d)
	}
}

func TestConnectReportsStartFailure(t *testing.T) {
	boom := errors.New("boom")
	_, err := Connect(socketPath(t), func() error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("Connect error = %v, want the start error", err)
	}
}
