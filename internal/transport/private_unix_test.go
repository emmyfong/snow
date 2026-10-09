//go:build !windows

package transport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSocketFolderPrivate(t *testing.T) {
	path := socketPath(t)
	listen(t, path)
	for _, c := range []struct {
		path string
		want os.FileMode
	}{{filepath.Dir(path), 0o700}, {path, 0o600}} {
		info, err := os.Stat(c.path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != c.want {
			t.Errorf("%s mode %o, want %o", c.path, got, c.want)
		}
	}
}

func TestPrivateFolderTightensLooseMode(t *testing.T) {
	path := socketPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	listen(t, path)
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("folder mode %o after Listen, want 700", got)
	}
}
