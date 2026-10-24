//go:build !windows

// Package privatetest checks in tests that a file or folder is private to
// the current user.
package privatetest

import (
	"os"
	"path/filepath"
	"testing"
)

// OwnerOnly fails t unless path has mode want, for example 0600 for a file
// or 0700 for a folder. On Windows it checks the access list instead.
func OwnerOnly(t testing.TB, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s mode %o, want %o", filepath.Base(path), got, want)
	}
}
