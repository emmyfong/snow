package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emmyfong/snow/internal/gittest"
)

// isolate keeps the tests away from the user's git config.
func isolate(t *testing.T) { gittest.Isolate(t) }

// runGit runs git in dir for a test and returns its trimmed output.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return gittest.Run(t, dir, args...)
}

// newEmptyRepo is a repository with no commit.
func newEmptyRepo(t *testing.T) Repo {
	t.Helper()
	isolate(t)
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	return Repo{Path: dir}
}

// newRepo is a repository with one commit on main.
func newRepo(t *testing.T) Repo {
	t.Helper()
	r := newEmptyRepo(t)
	if err := os.WriteFile(filepath.Join(r.Path, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, r.Path, "add", "a.txt")
	runGit(t, r.Path, "commit", "-q", "-m", "first")
	return r
}

// readFile returns a file's bytes, or nil when it is not there.
func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return b
}
