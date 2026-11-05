package git

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/emmyfong/snow/internal/gittest"
)

// TestGitStageCommit: one staged file is committed; the other change stays
// unstaged.
func TestGitStageCommit(t *testing.T) {
	dir := repo(t)
	ctx := context.Background()
	write(t, dir, "a.txt", "changed\n")
	write(t, dir, "b.txt", "new\n")
	r := Repo{Path: dir}
	if err := r.Stage(ctx, "b.txt"); err != nil {
		t.Fatal(err)
	}
	hash, err := r.Commit(ctx, "add b")
	if err != nil {
		t.Fatal(err)
	}
	if head := gittest.Run(t, dir, "rev-parse", "--short", "HEAD"); hash != head {
		t.Errorf("Commit returned %q, HEAD is %q", hash, head)
	}
	if files := gittest.Run(t, dir, "show", "--name-only", "--format=", "HEAD"); files != "b.txt" {
		t.Errorf("the commit holds %q, want b.txt", files)
	}
	st, err := r.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Changes) != 1 || st.Changes[0] != (Change{Path: "a.txt", Work: "M"}) {
		t.Errorf("after the commit: %+v, want a.txt unstaged", st.Changes)
	}
}

// TestGitCommitMessage: the message reaches git whole, even with a leading
// dash, and Snow adds no trailer.
func TestGitCommitMessage(t *testing.T) {
	dir := repo(t)
	ctx := context.Background()
	write(t, dir, "b.txt", "b\n")
	r := Repo{Path: dir}
	if err := r.Stage(ctx, "b.txt"); err != nil {
		t.Fatal(err)
	}
	msg := "-n looks like an option\n\nSecond paragraph."
	if _, err := r.Commit(ctx, msg); err != nil {
		t.Fatal(err)
	}
	if got := gittest.Run(t, dir, "log", "-1", "--format=%B"); got != msg {
		t.Errorf("message %q, want %q", got, msg)
	}
}

// TestGitCommitEmpty: an empty message is refused before git runs, and a
// commit with nothing staged fails with git's own words.
func TestGitCommitEmpty(t *testing.T) {
	dir := repo(t)
	r := Repo{Path: dir}
	if _, err := r.Commit(context.Background(), "  \n"); !errors.Is(err, ErrEmptyMessage) {
		t.Errorf("empty message: %v", err)
	}
	_, err := r.Commit(context.Background(), "nothing")
	var ge *Error
	if !errors.As(err, &ge) || ge.Sub != "commit" {
		t.Errorf("nothing staged: %v, want git's commit failure", err)
	}
}

// TestGitUnstage: a staged file goes back to unstaged, keeping its change,
// and on a branch with no commit too.
func TestGitUnstage(t *testing.T) {
	ctx := context.Background()
	dir := repo(t)
	write(t, dir, "a.txt", "changed\n")
	r := Repo{Path: dir}
	if err := r.Stage(ctx, "a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := r.Unstage(ctx, "a.txt"); err != nil {
		t.Fatal(err)
	}
	st, _ := r.Status(ctx)
	if len(st.Changes) != 1 || st.Changes[0] != (Change{Path: "a.txt", Work: "M"}) {
		t.Errorf("after unstage: %+v", st.Changes)
	}

	empty := t.TempDir()
	gittest.Run(t, empty, "init", "-q", "-b", "main")
	write(t, empty, "x.txt", "x\n")
	e := Repo{Path: empty}
	if err := e.Stage(ctx, "x.txt"); err != nil {
		t.Fatal(err)
	}
	if err := e.Unstage(ctx, "x.txt"); err != nil {
		t.Fatalf("unstage with no commit: %v", err)
	}
	if st, _ := e.Status(ctx); len(st.Changes) != 1 || !st.Changes[0].Untracked() {
		t.Errorf("after unstage with no commit: %+v", st.Changes)
	}
}

// TestGitStageLiteral: a file whose name looks like a wildcard stages only
// itself.
func TestGitStageLiteral(t *testing.T) {
	ctx := context.Background()
	dir := repo(t)
	write(t, dir, "x.txt", "x\n")
	write(t, dir, "[x].txt", "y\n")
	r := Repo{Path: dir}
	if err := r.Stage(ctx, "[x].txt"); err != nil {
		t.Fatal(err)
	}
	if staged := gittest.Run(t, dir, "diff", "--cached", "--name-only"); staged != "[x].txt" {
		t.Errorf("staged %q, want only [x].txt", staged)
	}
}

// TestGitNeverChangesConfig: staging, committing, and reading leave the
// repository's and the global config byte for byte as they were.
func TestGitNeverChangesConfig(t *testing.T) {
	ctx := context.Background()
	dir := repo(t)
	global := os.Getenv("GIT_CONFIG_GLOBAL")
	if err := os.WriteFile(global, []byte("[core]\n\tautocrlf = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(dir, ".git", "config")
	before := map[string][]byte{global: read(t, global), local: read(t, local)}

	write(t, dir, "b.txt", "b\n")
	r := Repo{Path: dir}
	if err := r.Stage(ctx, "b.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Diff(ctx, Change{Path: "b.txt", Index: "A"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Status(ctx); err != nil {
		t.Fatal(err)
	}
	for path, was := range before {
		if now := read(t, path); !bytes.Equal(now, was) {
			t.Errorf("%s changed:\n%s\nwas:\n%s", path, now, was)
		}
	}
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestGitStageAll: stage all reaches every change of the repository from a
// subfolder: a new file, a deleted one, and one above the folder.
func TestGitStageAll(t *testing.T) {
	dir := repo(t)
	ctx := context.Background()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "sub/new.txt", "new\n")
	write(t, dir, "top.txt", "top\n")
	if err := os.Remove(filepath.Join(dir, "a.txt")); err != nil {
		t.Fatal(err)
	}
	r := Repo{Path: filepath.Join(dir, "sub")}
	if err := r.StageAll(ctx); err != nil {
		t.Fatal(err)
	}
	st, err := r.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Changes) != 3 {
		t.Fatalf("changes %+v, want 3", st.Changes)
	}
	for _, c := range st.Changes {
		if !c.Staged() || c.Work != "" {
			t.Errorf("%+v is not wholly staged", c)
		}
	}
}
