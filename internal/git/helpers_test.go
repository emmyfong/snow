package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emmyfong/snow/internal/gittest"
)

// write writes body to name in dir, making its folders.
func write(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// repo returns a new repository with one commit of a.txt on main.
func repo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gittest.Repo(t, dir)
	return dir
}
