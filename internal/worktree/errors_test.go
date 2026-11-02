package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// TestRemoveLockedWorktree: a worktree that git locks is refused with its own
// error, and stays.
func TestRemoveLockedWorktree(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	wt, err := r.Create(ctx, "feat/a")
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, r.Path, "worktree", "lock", "--reason", "in use", wt.Path)
	if err := r.Remove(ctx, wt.Path); !errors.Is(err, ErrLocked) {
		t.Fatalf("error %v, want ErrLocked", err)
	}
	if _, err := os.Stat(wt.Path); err != nil {
		t.Fatalf("the locked worktree is gone: %v", err)
	}
}

// TestRemoveMissingWorktree: a worktree whose folder was deleted by hand has
// nothing left to lose, so Remove clears git's entry and keeps the branch.
func TestRemoveMissingWorktree(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	wt, err := r.Create(ctx, "feat/a")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(wt.Path); err != nil {
		t.Fatal(err)
	}
	if err := r.Remove(ctx, wt.Path); err != nil {
		t.Fatalf("Remove of a missing worktree: %v", err)
	}
	list, err := r.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list %+v, %v", list, err)
	}
	if got := runGit(t, r.Path, "branch", "--list", "feat/a"); got == "" {
		t.Error("the branch is gone")
	}
	if _, err := r.Create(ctx, "feat/b"); err != nil {
		t.Fatalf("Create after the cleanup: %v", err)
	}
}

// TestCancelledContext: a cancelled call says so. It is not a bad branch
// name or a repository with no commits, which are what a failed yes-or-no
// question to git would mean.
func TestCancelledContext(t *testing.T) {
	r := newRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Create(ctx, "feat/a")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v, want context.Canceled", err)
	}
	for _, not := range []error{ErrBadBranch, ErrNoCommits, ErrBranchExists, ErrNotRepo} {
		if errors.Is(err, not) {
			t.Errorf("a cancelled call reads as %v", not)
		}
	}
}

// TestDeclined: only git's exit code 1 is an answer of "no" to a question.
// A crash, a kill, or a failure to start is a failure, and no sentinel
// error may hide it.
func TestDeclined(t *testing.T) {
	exit := func(code int) error {
		cmd := exec.Command("git", "check-ref-format", "refs/heads/bad..name")
		if code != 1 {
			cmd = exec.Command("git", "--no-such-option")
		}
		err := cmd.Run()
		var e *exec.ExitError
		if !errors.As(err, &e) {
			t.Fatalf("git did not fail: %v", err)
		}
		return &failure{sub: "x", code: e.ExitCode(), cause: err}
	}
	if !declined(exit(1)) {
		t.Error("exit code 1 is a no")
	}
	if declined(exit(129)) {
		t.Error("exit code 129 is a failure")
	}
	if declined(&failure{sub: "x", code: -1, cause: context.Canceled}) || declined(fmt.Errorf("plain")) {
		t.Error("a kill or a plain error is a failure")
	}
}
