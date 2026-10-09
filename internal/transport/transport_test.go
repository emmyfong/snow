package transport

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

// socketPath returns a socket path in a short temp folder. t.TempDir paths
// include the test name and can pass the OS limit on socket path length.
func socketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "snow")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "run", "default.sock")
}

func listen(t *testing.T, path string) net.Listener {
	t.Helper()
	l, err := Listen(path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func TestTransportRoundTrip(t *testing.T) {
	path := socketPath(t)
	l := listen(t, path)

	got := make(chan api.Message, 1)
	go func() {
		c, err := l.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = c.Close() }()
		conn := api.NewConn(c)
		m, err := conn.Receive()
		if err != nil {
			t.Error(err)
			return
		}
		got <- m
		if err := conn.Send(&api.Welcome{ServerVersion: "test", Protocol: api.ProtocolVersion}); err != nil {
			t.Error(err)
		}
	}()

	c, err := Dial(path)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = c.Close() }()
	conn := api.NewConn(c)
	hello := &api.Hello{ClientVersion: "test", Protocol: api.ProtocolVersion, Cols: 80, Rows: 24}
	if err := conn.Send(hello); err != nil {
		t.Fatal(err)
	}
	if m := <-got; !reflect.DeepEqual(m, hello) {
		t.Fatalf("server received %#v, want %#v", m, hello)
	}
	reply, err := conn.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if w, ok := reply.(*api.Welcome); !ok || w.ServerVersion != "test" {
		t.Fatalf("client received %#v, want Welcome", reply)
	}
}

func TestListenWhileServerRunning(t *testing.T) {
	path := socketPath(t)
	listen(t, path)
	if _, err := Listen(path); !errors.Is(err, ErrInUse) {
		t.Fatalf("second Listen error = %v, want ErrInUse", err)
	}
}

// staleSocket leaves a socket file at path with nobody listening, as a
// crashed server does.
func staleSocket(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	_ = l.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stale socket file missing: %v", err)
	}
}

func TestListenReplacesStaleSocket(t *testing.T) {
	path := socketPath(t)
	staleSocket(t, path)
	listen(t, path)
}

func TestConcurrentListenOneWins(t *testing.T) {
	for round := range 50 {
		path := socketPath(t)
		staleSocket(t, path)
		const n = 20
		var wg sync.WaitGroup
		var mu sync.Mutex
		var won []net.Listener
		var other []error
		for range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				l, err := Listen(path)
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					won = append(won, l)
				case !errors.Is(err, ErrInUse):
					other = append(other, err)
				}
			}()
		}
		wg.Wait()
		for _, l := range won {
			_ = l.Close()
		}
		if len(won) != 1 || len(other) != 0 {
			t.Fatalf("round %d: %d servers started, unexpected errors %v; want exactly one and the rest ErrInUse", round, len(won), other)
		}
	}
}

func TestListenKeepsRegularFile(t *testing.T) {
	path := socketPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("precious"), 0o600); err != nil {
		t.Fatal(err)
	}
	if l, err := Listen(path); err == nil {
		_ = l.Close()
		t.Fatal("Listen replaced a regular file")
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "precious" {
		t.Fatalf("regular file changed: %q, %v", b, err)
	}
}

func TestDialWithoutServer(t *testing.T) {
	if _, err := Dial(socketPath(t)); !errors.Is(err, ErrNoServer) {
		t.Fatalf("Dial error = %v, want ErrNoServer", err)
	}
}

func TestListenPathTooLong(t *testing.T) {
	path := filepath.Join(os.TempDir(), strings.Repeat("x", 120), "default.sock")
	if _, err := Listen(path); !errors.Is(err, ErrPathTooLong) {
		t.Fatalf("Listen error = %v, want ErrPathTooLong", err)
	}
}
