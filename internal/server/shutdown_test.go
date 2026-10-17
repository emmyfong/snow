package server

import (
	"net"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/pkg/api"
)

// TestRestartSnapshotsAll: a newer client, on another protocol, asks the
// server to stop; the server saves every session first, so they come back
// as saved in the new server.
func TestRestartSnapshotsAll(t *testing.T) {
	opts := testOptions()
	opts.SessionsDir = t.TempDir()
	opts.saveDelay = time.Hour // only the stop itself may save
	opts.busyEvery = time.Hour
	s := New(opts)
	defer s.Close()
	for _, name := range []string{"a", "b"} {
		if _, err := s.CreateSession(name); err != nil {
			t.Fatal(err)
		}
	}
	a, b := net.Pipe()
	go s.serveConn(b)
	conn := api.NewConn(a)
	defer func() { _ = a.Close() }()
	if _, err := api.ClientHandshake(conn, api.Hello{ClientVersion: "next", Protocol: api.ProtocolVersion + 1}); err == nil {
		t.Fatal("handshake on another protocol succeeded")
	}
	if err := conn.Send(&api.Shutdown{}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the server did not stop")
	}
	s.Close()
	st, err := store.Open(opts.SessionsDir)
	if err != nil {
		t.Fatal(err)
	}
	if infos, _ := st.List(); len(infos) != 2 {
		t.Fatalf("saved %v, want both sessions", infos)
	}
}

// TestMismatchWithoutShutdownKeepsServer: a client on another protocol that
// does not ask to stop leaves the server running.
func TestMismatchWithoutShutdownKeepsServer(t *testing.T) {
	s := newTestServer(t)
	if _, err := s.CreateSession("a"); err != nil {
		t.Fatal(err)
	}
	a, b := net.Pipe()
	go s.serveConn(b)
	conn := api.NewConn(a)
	_, _ = api.ClientHandshake(conn, api.Hello{ClientVersion: "next", Protocol: api.ProtocolVersion + 1})
	_ = a.Close()
	select {
	case <-s.Done():
		t.Fatal("the server stopped without being asked")
	case <-time.After(200 * time.Millisecond):
	}
}
