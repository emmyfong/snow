package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// isolate keeps the tests away from the user's git config and gives git an
// identity through the environment, so no test writes config.
func isolate(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(home, "gitconfig-system"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, who := range []string{"AUTHOR", "COMMITTER"} {
		t.Setenv("GIT_"+who+"_NAME", "Test")
		t.Setenv("GIT_"+who+"_EMAIL", "test@example.com")
	}
}

// runGit runs git in dir for a test and returns its trimmed output.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
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
