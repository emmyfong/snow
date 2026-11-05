package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestDiscard: a worktree nobody worked in goes with its branch, so a
// second Create with the same name works.
func TestDiscard(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	wt, err := r.Create(ctx, "agent/1")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Discard(ctx, wt); err != nil {
		t.Fatal(err)
	}
	if got := runGit(t, r.Path, "branch", "--list", "agent/1"); got != "" {
		t.Fatalf("the branch is still there: %q", got)
	}
	if _, err := r.Create(ctx, "agent/1"); err != nil {
		t.Fatalf("create again: %v", err)
	}
}

// TestDiscardKeepsWork: a branch with a new commit is kept, and a worktree
// with changes is not removed.
func TestDiscardKeepsWork(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	wt, err := r.Create(ctx, "agent/1")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(wt.Path, "new.txt"), "work")
	if err := r.Discard(ctx, wt); !errors.Is(err, ErrDirty) {
		t.Fatalf("discard with changes: %v, want ErrDirty", err)
	}
	runGit(t, wt.Path, "add", "new.txt")
	runGit(t, wt.Path, "commit", "-q", "-m", "work")
	if err := r.Discard(ctx, wt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Errorf("the worktree folder is still there: %v", err)
	}
	if got := runGit(t, r.Path, "branch", "--list", "agent/1"); got == "" {
		t.Error("the branch with a commit is gone")
	}
}
