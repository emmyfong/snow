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

// TestOldServerIsNamed: a new snow that meets a server on an older protocol
// says so and asks before restarting it, instead of attaching and
// misreading its messages. Declining keeps the old server.
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
	if !strings.Contains(out.String(), "server is version old") || !strings.Contains(out.String(), "Keeping the running server") {
		t.Fatalf("stdout %q, want the old version named and the server kept", out.String())
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
		for i := 0; ; i++ {
			raw, err := l.Accept()
			if err != nil {
				return
			}
			older := api.Welcome{ServerVersion: "dev-0ld", Protocol: api.ProtocolVersion}
			if _, err := api.ServerHandshake(api.NewConn(raw), older); err == nil && i == 1 {
				close(attached)
			}
			_ = raw.Close()
		}
	}()

	stdin = strings.NewReader("n\n")
	t.Cleanup(func() { stdin = os.Stdin })
	var out, errOut bytes.Buffer
	run([]string{"work"}, &out, &errOut)
	if !strings.Contains(out.String(), "server is version dev-0ld") || !strings.Contains(out.String(), "Keeping the running server") {
		t.Fatalf("stdout %q, want the other build named and the server kept", out.String())
	}
	select {
	case <-attached:
	case <-time.After(5 * time.Second):
		t.Fatalf("the client did not attach to the kept server; stderr %q", errOut.String())
	}
}
