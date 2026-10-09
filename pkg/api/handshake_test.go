package api

import (
	"errors"
	"net"
	"testing"
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
	if !errors.As(err, &mismatch) || mismatch.ClientVersion != "0.2.0" || mismatch.ServerVersion != "0.1.0" {
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
	if r := <-res; !errors.Is(r.err, ErrUnexpectedMessage) {
		t.Fatalf("server error = %v, want ErrUnexpectedMessage", r.err)
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
