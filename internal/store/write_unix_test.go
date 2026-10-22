//go:build !windows

package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emmyfong/snow/internal/private"
)

func perm(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func TestSnapshotFileMode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := perm(t, dir); got != 0o700 {
		t.Fatalf("folder mode %o, want 700", got)
	}

	path := filepath.Join(dir, "api.json")
	// A loose file left by a user and a loose stale temp file must not leak
	// their modes into the snapshot.
	for _, p := range []string{path, private.TempPath(path)} {
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Save(sample("api")); err != nil {
		t.Fatal(err)
	}
	if got := perm(t, path); got != 0o600 {
		t.Fatalf("snapshot mode %o, want 600", got)
	}
}

// TestOpenTightensFolder: saved sessions hold terminal output, so an
// existing sessions folder that others can read is made private.
func TestOpenTightensFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(dir); err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("folder mode %v, %v; want 0700", fi.Mode().Perm(), err)
	}
}
