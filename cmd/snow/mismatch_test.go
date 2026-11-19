package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/transport"
	"github.com/emmyfong/snow/pkg/api"
)

// TestOldServerIsNamed: a protocol mismatch shows the restart popup before
// Snow exits with the mismatch error.
func TestOldServerIsNamed(t *testing.T) {
	dir, err := os.MkdirTemp("", "snow")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("LOCALAPPDATA", dir)
	path, err := transport.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	l, err := transport.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		raw, err := l.Accept()
		if err != nil {
			return
		}
		defer func() { _ = raw.Close() }()
		old := api.Welcome{ServerVersion: "old", Protocol: api.ProtocolVersion - 1}
		_, _ = api.ServerHandshake(api.NewConn(raw), old)
	}()

	stdin = strings.NewReader("n\n")
	t.Cleanup(func() { stdin = os.Stdin })
	var out, errOut bytes.Buffer
	if code := run([]string{"work"}, &out, &errOut); code != 1 {
		t.Fatalf("exit code %d, want 1; stderr %q", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "api: client") || !strings.Contains(errOut.String(), "cannot talk to server") {
		t.Fatalf("stderr %q, want the protocol mismatch error", errOut.String())
	}
}

// TestOtherServerBuildIsNamed: a server of another build on the same
// protocol, such as one left running from before an update, is named and
// the user is asked to restart it. Declining attaches to it anyway, since
// the protocol still matches.
func TestOtherServerBuildIsNamed(t *testing.T) {
	dir, err := os.MkdirTemp("", "snow")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("LOCALAPPDATA", dir)
	path, err := transport.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	l, err := transport.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	attached := make(chan struct{})
	go func() {
		raw, err := l.Accept()
		if err != nil {
			return
		}
		defer func() { _ = raw.Close() }()
		conn := api.NewConn(raw)
		older := api.Welcome{ServerVersion: "dev-0ld", Protocol: api.ProtocolVersion}
		if _, err := api.ServerHandshake(conn, older); err != nil {
			return
		}
		msg, _ := conn.Receive()
		if attach, ok := msg.(*api.Attach); ok && attach.Session == "work" {
			close(attached)
		}
	}()

	stdin = strings.NewReader("n\n")
	t.Cleanup(func() { stdin = os.Stdin })
	var out, errOut bytes.Buffer
	run([]string{"work"}, &out, &errOut)
	select {
	case <-attached:
	case <-time.After(5 * time.Second):
		t.Fatalf("the client did not attach to the kept server; stderr %q", errOut.String())
	}
}
