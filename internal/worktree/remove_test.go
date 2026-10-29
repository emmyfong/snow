package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestRemoveCleanWorktree: a clean worktree goes, and its branch stays for
// the user to merge or delete.
func TestRemoveCleanWorktree(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	wt, err := r.Create(ctx, "feat/a")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Remove(ctx, wt.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Errorf("the worktree folder is still there: %v", err)
	}
	if got := runGit(t, r.Path, "branch", "--list", "feat/a"); got == "" {
		t.Error("the branch is gone")
	}
	list, err := r.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list %+v, %v", list, err)
	}
}

// TestWorktreeWithChangesKept: a worktree with a changed file, a new file,
// or a staged file is never removed. An ignored file is not work, so it does
// not stop the removal.
func TestWorktreeWithChangesKept(t *testing.T) {
	tests := []struct {
		name   string
		change func(t *testing.T, wt Worktree)
		want   error
	}{
		{"changed tracked file", func(t *testing.T, wt Worktree) {
			write(t, filepath.Join(wt.Path, "a.txt"), "changed\n")
		}, ErrDirty},
		{"new file", func(t *testing.T, wt Worktree) {
			write(t, filepath.Join(wt.Path, "new.txt"), "new\n")
		}, ErrDirty},
		{"new file in a new folder", func(t *testing.T, wt Worktree) {
			dir := filepath.Join(wt.Path, "d", "e")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(dir, "f.txt"), "f\n")
		}, ErrDirty},
		{"staged file", func(t *testing.T, wt Worktree) {
			write(t, filepath.Join(wt.Path, "s.txt"), "s\n")
			runGit(t, wt.Path, "add", "s.txt")
		}, ErrDirty},
		{"committed work", func(t *testing.T, wt Worktree) {
			write(t, filepath.Join(wt.Path, "c.txt"), "c\n")
			runGit(t, wt.Path, "add", "c.txt")
			runGit(t, wt.Path, "commit", "-q", "-m", "work")
		}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRepo(t)
			ctx := context.Background()
			wt, err := r.Create(ctx, "feat/a")
			if err != nil {
				t.Fatal(err)
			}
			tt.change(t, wt)
			err = r.Remove(ctx, wt.Path)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error %v, want %v", err, tt.want)
			}
			_, statErr := os.Stat(wt.Path)
			if kept := statErr == nil; kept != (tt.want != nil) {
				t.Fatalf("worktree kept = %v with error %v", kept, err)
			}
		})
	}
}

// TestRemoveIgnoredFilesDoNotBlock: build output that git ignores goes with
// the worktree.
func TestRemoveIgnoredFilesDoNotBlock(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	wt, err := r.Create(ctx, "feat/a")
	if err != nil {
		t.Fatal(err)
	}
	exclude := filepath.Join(r.Path, ".git", "info", "exclude")
	write(t, exclude, string(readFile(t, exclude))+"*.out\n")
	write(t, filepath.Join(wt.Path, "build.out"), "x")
	if err := r.Remove(ctx, wt.Path); err != nil {
		t.Fatal(err)
	}
}

// TestRemoveRefuses: a path that is not a worktree of the repository, and
// the main checkout, are never removed.
func TestRemoveRefuses(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	list, err := r.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	other := newRepo(t)
	otherWT, err := other.Create(ctx, "feat/o")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		path string
		want error
	}{
		{"main checkout", list[0].Path, ErrMainWorktree},
		{"folder of the repository", filepath.Join(r.Path, "nope"), ErrNotWorktree},
		{"worktree of another repository", otherWT.Path, ErrNotWorktree},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := r.Remove(ctx, tt.path); !errors.Is(err, tt.want) {
				t.Fatalf("error %v, want %v", err, tt.want)
			}
		})
	}
	if _, err := os.Stat(otherWT.Path); err != nil {
		t.Fatalf("the other repository's worktree is gone: %v", err)
	}
	if _, err := os.Stat(list[0].Path); err != nil {
		t.Fatalf("the main checkout is gone: %v", err)
	}
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
