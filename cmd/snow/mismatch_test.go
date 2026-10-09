package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/transport"
	"github.com/emmyfong/snow/pkg/api"
)

// TestOldServerIsNamed: a new snow that meets a server on an older protocol
// says so, instead of attaching and misreading its messages.
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

	var out, errOut bytes.Buffer
	if code := run([]string{"work"}, &out, &errOut); code != 1 {
		t.Fatalf("exit code %d, want 1; stderr %q", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "server is version old") {
		t.Fatalf("stderr %q, want it to name the old server's version", errOut.String())
	}
}
