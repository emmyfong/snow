//go:build !windows

package convstore

import (
	"io/fs"
	"os"
	"testing"
)

// makeUnreadable makes the file at path one the user cannot open, and
// returns the error that a read of it wraps.
func makeUnreadable(t *testing.T, path string) error {
	t.Helper()
	if os.Getuid() == 0 {
		t.Skip("root reads every file")
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	return fs.ErrPermission
}
