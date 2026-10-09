package api

import (
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

func pair(t *testing.T) (client, server *Conn) {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })
	return NewConn(a), NewConn(b)
}

type serverResult struct {
	hello *Hello
	err   error
}

func runServer(server *Conn, w Welcome) <-chan serverResult {
	out := make(chan serverResult, 1)
	go func() {
		h, err := ServerHandshake(server, w)
		out <- serverResult{h, err}
	}()
	return out
}

func TestHandshakeMatch(t *testing.T) {
	client, server := pair(t)
	res := runServer(server, Welcome{ServerVersion: "0.1.0", Protocol: ProtocolVersion})
	w, err := ClientHandshake(client, Hello{ClientVersion: "0.1.0", Protocol: ProtocolVersion, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	if w.ServerVersion != "0.1.0" {
		t.Fatalf("client got %+v", w)
	}
	r := <-res
	if r.err != nil || r.hello.Cols != 80 {
		t.Fatalf("server got %+v, %v", r.hello, r.err)
	}
}

func TestHandshakeMismatch(t *testing.T) {
	client, server := pair(t)
	res := runServer(server, Welcome{ServerVersion: "0.1.0", Protocol: 1})
	w, err := ClientHandshake(client, Hello{ClientVersion: "0.2.0", Protocol: 2})
	if !errors.Is(err, ErrVersionMismatch) {
		t.Fatalf("client error = %v, want ErrVersionMismatch", err)
	}
	var mismatch *VersionMismatchError
	if !errors.As(err, &mismatch) || mismatch.ClientVersion != "0.2.0" || mismatch.ServerVersion != "0.1.0" ||
		mismatch.ClientProtocol != 2 || mismatch.ServerProtocol != 1 {
		t.Fatalf("client error = %#v, want both versions", err)
	}
	if w == nil || w.ServerVersion != "0.1.0" {
		t.Fatalf("client should still get the server's Welcome, got %+v", w)
	}
	if r := <-res; !errors.Is(r.err, ErrVersionMismatch) {
		t.Fatalf("server error = %v, want ErrVersionMismatch", r.err)
	}
}

func TestHandshakeUnexpectedFirstMessage(t *testing.T) {
	client, server := pair(t)
	res := runServer(server, Welcome{ServerVersion: "0.1.0", Protocol: ProtocolVersion})
	if err := client.Send(&Bell{Pane: 1}); err != nil {
		t.Fatal(err)
	}
	// The client learns why instead of seeing a bare disconnect.
	reply, err := client.Receive()
	if err != nil {
		t.Fatalf("client got no reply: %v", err)
	}
	if e, ok := reply.(*Error); !ok || e.Code != CodeBadHandshake {
		t.Fatalf("client got %#v, want Error{bad-handshake}", reply)
	}
	if r := <-res; !errors.Is(r.err, ErrUnexpectedMessage) {
		t.Fatalf("server error = %v, want ErrUnexpectedMessage", r.err)
	}
}

func TestServerHandshakeTimesOut(t *testing.T) {
	defer setHandshakeTimeout(50 * time.Millisecond)()
	_, server := pair(t)
	select {
	case r := <-runServer(server, Welcome{Protocol: ProtocolVersion}):
		if !errors.Is(r.err, os.ErrDeadlineExceeded) {
			t.Fatalf("server error = %v, want a deadline error", r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ServerHandshake waited forever for a silent client")
	}
}

func TestClientHandshakeTimesOut(t *testing.T) {
	defer setHandshakeTimeout(50 * time.Millisecond)()
	client, server := pair(t)
	go func() { _, _ = server.Receive() }() // read Hello, never answer
	done := make(chan error, 1)
	go func() {
		_, err := ClientHandshake(client, Hello{Protocol: ProtocolVersion})
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("client error = %v, want a deadline error", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ClientHandshake waited forever for a silent server")
	}
}

func TestClientHandshakeServerError(t *testing.T) {
	client, server := pair(t)
	go func() {
		_, _ = server.Receive()
		_ = server.Send(&Error{Code: "busy", Message: "server is shutting down"})
	}()
	_, err := ClientHandshake(client, Hello{Protocol: ProtocolVersion})
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != "busy" {
		t.Fatalf("client error = %v, want the server's Error", err)
	}
}
