package apitest

import (
	"os"
	"path/filepath"
	"testing"
)

// SocketPath returns a socket path in a new temporary folder that is removed
// when the test ends. It avoids t.TempDir: macOS temp paths are long, and a
// Unix socket path must stay under about 100 bytes.
func SocketPath(t testing.TB) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "snow")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "run", "default.sock")
}
