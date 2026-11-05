// Package gittest gives tests real git repositories that stay away from the
// user's git config. It is for tests only.
package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Isolate keeps a test away from the user's git config and gives git an
// identity through the environment, so no test writes config. It sets
// environment variables, so a test that calls it cannot run in parallel.
func Isolate(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(home, "gitconfig-system"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	// A git run inside a hook or a wrapper leaves these set, and they point
	// every command at the wrong repository. Setenv first, so the old value
	// comes back when the test ends.
	for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR"} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	// A repository above the temp folder must not be found by accident.
	t.Setenv("GIT_CEILING_DIRECTORIES", os.TempDir())
	for _, who := range []string{"AUTHOR", "COMMITTER"} {
		t.Setenv("GIT_"+who+"_NAME", "Test")
		t.Setenv("GIT_"+who+"_EMAIL", "test@example.com")
	}
}

// Run runs git in dir and returns its trimmed output. A failure fails t.
func Run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// Repo isolates t and returns a new repository in dir with one commit on
// main.
func Repo(t *testing.T, dir string) {
	t.Helper()
	Isolate(t)
	Run(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	Run(t, dir, "add", "a.txt")
	Run(t, dir, "commit", "-q", "-m", "first")
}
