package transport

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestListenReplacesStaleSocket(t *testing.T) {
	path := socketPath(t)
	l, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a crashed server: the socket file stays, nobody listens.
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	_ = l.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stale socket file missing: %v", err)
	}
	listen(t, path)
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

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "default.sock" || len(path) > maxPathLen {
		t.Fatalf("DefaultPath() = %q", path)
	}
}
