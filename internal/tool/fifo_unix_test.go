//go:build !windows

package tool

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestReadRefusesFIFO: read refuses a named pipe instead of blocking on it.
func TestReadRefusesFIFO(t *testing.T) {
	env := tree(t, nil)
	if err := syscall.Mkfifo(filepath.Join(env.Folder.Path, "pipe"), 0o600); err != nil {
		t.Skip(err)
	}
	if got := run(t, env, "read", `{"path":"pipe"}`); !got.IsError || !strings.Contains(got.Content, "not a regular file") {
		t.Fatalf("got %+v", got)
	}
}
