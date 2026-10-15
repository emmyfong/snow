package server

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// failingListener fails every Accept, as when the process is out of file
// descriptors, until it is closed.
type failingListener struct {
	calls  atomic.Int64
	closed chan struct{}
	once   sync.Once
}

func (l *failingListener) Accept() (net.Conn, error) {
	l.calls.Add(1)
	select {
	case <-l.closed:
		return nil, net.ErrClosed
	default:
		return nil, errors.New("accept4: too many open files")
	}
}

func (l *failingListener) Close() error   { l.once.Do(func() { close(l.closed) }); return nil }
func (l *failingListener) Addr() net.Addr { return &net.UnixAddr{Name: "fake", Net: "unix"} }

// TestAcceptBacksOff: a listener that keeps failing must not spin the CPU or
// flood the log.
func TestAcceptBacksOff(t *testing.T) {
	s := newTestServer(t)
	l := &failingListener{closed: make(chan struct{})}
	done := make(chan struct{})
	go func() { s.accept(l); close(done) }()
	time.Sleep(500 * time.Millisecond)
	_ = l.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("accept did not return after the listener closed")
	}
	if n := l.calls.Load(); n > 20 {
		t.Fatalf("%d Accept calls in 0.5 s, want a backoff", n)
	}
}
