package worktree

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCreateWorktree: the worktree exists on a new branch that starts at the
// commit the main checkout is on, in the folder the name says.
func TestCreateWorktree(t *testing.T) {
	r := newRepo(t)
	base := runGit(t, r.Path, "rev-parse", "HEAD")
	wt, err := r.Create(context.Background(), "feat/login")
	if err != nil {
		t.Fatal(err)
	}
	if want := ".worktrees/feat-login"; !strings.HasSuffix(filepath.ToSlash(wt.Path), want) {
		t.Fatalf("path %s does not end in %s", wt.Path, want)
	}
	if fi, err := os.Stat(wt.Path); err != nil || !fi.IsDir() {
		t.Fatalf("no worktree folder: %v", err)
	}
	if got := runGit(t, wt.Path, "rev-parse", "--abbrev-ref", "HEAD"); got != "feat/login" || wt.Branch != "feat/login" {
		t.Fatalf("branch %q, returned %q", got, wt.Branch)
	}
	if wt.Head != base || runGit(t, wt.Path, "rev-parse", "HEAD") != base {
		t.Fatalf("head %q, want %q", wt.Head, base)
	}
}

// TestCreateLeavesMainAlone: the main checkout keeps its branch, commit,
// index, and clean status, so the user's work is not disturbed.
func TestCreateLeavesMainAlone(t *testing.T) {
	r := newRepo(t)
	runGit(t, r.Path, "status") // let git refresh the index before the snapshot
	index := filepath.Join(r.Path, ".git", "index")
	before := readFile(t, index)
	head := runGit(t, r.Path, "rev-parse", "HEAD")
	if _, err := r.Create(context.Background(), "feat/a"); err != nil {
		t.Fatal(err)
	}
	if got := runGit(t, r.Path, "rev-parse", "--abbrev-ref", "HEAD"); got != "main" {
		t.Errorf("main checkout is on %q", got)
	}
	if got := runGit(t, r.Path, "rev-parse", "HEAD"); got != head {
		t.Errorf("main checkout moved from %s to %s", head, got)
	}
	if got := runGit(t, r.Path, "status", "--porcelain"); got != "" {
		t.Errorf("the worktree shows in git status:\n%s", got)
	}
	if !bytes.Equal(before, readFile(t, index)) {
		t.Error("the main checkout's index changed")
	}
}

// TestGitConfigUntouched: Snow writes no git config, the repository's or
// the user's.
func TestGitConfigUntouched(t *testing.T) {
	r := newRepo(t)
	cfg := filepath.Join(r.Path, ".git", "config")
	before := readFile(t, cfg)
	global := os.Getenv("GIT_CONFIG_GLOBAL")
	for _, branch := range []string{"feat/a", "feat/b"} {
		if _, err := r.Create(context.Background(), branch); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(before, readFile(t, cfg)) {
		t.Errorf("the repository config changed:\n%s", readFile(t, cfg))
	}
	if _, err := os.Stat(global); !os.IsNotExist(err) {
		t.Errorf("the user's git config was written: %v", err)
	}
}

// TestTwoWorktrees: two agents in one repository get two folders and two
// branches, and List shows the main checkout first.
func TestTwoWorktrees(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	a, err := r.Create(ctx, "feat/a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Create(ctx, "feat/b")
	if err != nil {
		t.Fatal(err)
	}
	if a.Path == b.Path || a.Branch == b.Branch {
		t.Fatalf("one worktree twice: %+v %+v", a, b)
	}
	list, err := r.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || !list[0].Main || list[0].Branch != "main" || list[1].Main || list[2].Main {
		t.Fatalf("list %+v", list)
	}
	got := map[string]bool{list[1].Branch: true, list[2].Branch: true}
	if !got["feat/a"] || !got["feat/b"] {
		t.Fatalf("branches in the list: %+v", list)
	}
}

// TestCreateFromElsewhere: a folder below the top, or a worktree, finds the
// main checkout, so every worktree sits in one place.
func TestCreateFromElsewhere(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	sub := filepath.Join(r.Path, "pkg", "x")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	a, err := Repo{Path: sub}.Create(ctx, "feat/a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Repo{Path: a.Path}.Create(ctx, "feat/b")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(a.Path) != filepath.Dir(b.Path) || !strings.HasSuffix(filepath.ToSlash(a.Path), ".worktrees/feat-a") {
		t.Fatalf("paths %s and %s", a.Path, b.Path)
	}
}

// TestCreateErrors: each failure a caller can act on has its own error.
func TestCreateErrors(t *testing.T) {
	tests := []struct {
		name   string
		repo   func(t *testing.T) Repo
		branch string
		want   error
	}{
		{"not a repository", func(t *testing.T) Repo { isolate(t); return Repo{Path: t.TempDir()} }, "feat/a", ErrNotRepo},
		{"folder not there", func(t *testing.T) Repo { isolate(t); return Repo{Path: filepath.Join(t.TempDir(), "gone")} }, "feat/a", ErrNotRepo},
		{"no commits", newEmptyRepo, "feat/a", ErrNoCommits},
		{"bad name", newRepo, "bad..name", ErrBadBranch},
		{"name with a dash first", newRepo, "-x", ErrBadBranch},
		{"empty name", newRepo, "", ErrBadBranch},
		{"branch exists", newRepo, "main", ErrBranchExists},
		{"folder exists", func(t *testing.T) Repo {
			r := newRepo(t)
			dir := filepath.Join(r.Path, ".worktrees", "taken")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			return r
		}, "taken", ErrPathExists},
		{"no git", func(t *testing.T) Repo {
			r := newRepo(t)
			t.Setenv("PATH", t.TempDir())
			return r
		}, "feat/a", ErrNoGit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.repo(t)
			_, err := r.Create(context.Background(), tt.branch)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error %v, want %v", err, tt.want)
			}
		})
	}
}

// TestFailedCreateLeavesNothing: a refused branch creates no folder.
func TestFailedCreateLeavesNothing(t *testing.T) {
	r := newRepo(t)
	if _, err := r.Create(context.Background(), "main"); !errors.Is(err, ErrBranchExists) {
		t.Fatalf("error %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.Path, ".worktrees")); !os.IsNotExist(err) {
		t.Fatalf("the refused create left a folder: %v", err)
	}
}
