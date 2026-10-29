package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestDetachedCommitsKept: a detached worktree with a commit that no branch
// has is never removed, though git status is clean. Once a branch holds the
// commit, or the worktree has no new commit, it goes.
func TestDetachedCommitsKept(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, wt Worktree)
		want  error
	}{
		{"detached, no new commit", func(t *testing.T, wt Worktree) {
			runGit(t, wt.Path, "checkout", "-q", "--detach")
		}, nil},
		{"detached with a commit on no branch", func(t *testing.T, wt Worktree) {
			runGit(t, wt.Path, "checkout", "-q", "--detach")
			write(t, filepath.Join(wt.Path, "d.txt"), "d\n")
			runGit(t, wt.Path, "add", "d.txt")
			runGit(t, wt.Path, "commit", "-q", "-m", "detached work")
		}, ErrUnreachableCommits},
		{"detached commit that a branch now holds", func(t *testing.T, wt Worktree) {
			runGit(t, wt.Path, "checkout", "-q", "--detach")
			write(t, filepath.Join(wt.Path, "d.txt"), "d\n")
			runGit(t, wt.Path, "add", "d.txt")
			runGit(t, wt.Path, "commit", "-q", "-m", "detached work")
			runGit(t, wt.Path, "branch", "saved")
		}, nil},
		{"detached commit that a tag holds", func(t *testing.T, wt Worktree) {
			runGit(t, wt.Path, "checkout", "-q", "--detach")
			write(t, filepath.Join(wt.Path, "d.txt"), "d\n")
			runGit(t, wt.Path, "add", "d.txt")
			runGit(t, wt.Path, "commit", "-q", "-m", "detached work")
			runGit(t, wt.Path, "tag", "saved")
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
			tt.setup(t, wt)
			err = r.Remove(ctx, wt.Path)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error %v, want %v", err, tt.want)
			}
			if _, statErr := os.Stat(wt.Path); (statErr == nil) != (tt.want != nil) {
				t.Fatalf("worktree kept = %v with error %v", statErr == nil, err)
			}
		})
	}
}

// TestFailedCreateLeavesNoBranch: a Create that git refuses after it made
// the branch takes the branch back, so the next try is not refused with
// ErrBranchExists. Two branch names that make one folder name are the
// common way to get there.
func TestFailedCreateLeavesNoBranch(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if _, err := r.Create(ctx, "feat/a"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Create(ctx, "feat-a"); !errors.Is(err, ErrPathExists) {
		t.Fatalf("second Create: %v, want ErrPathExists", err)
	}
	if got := runGit(t, r.Path, "branch", "--list", "feat-a"); got != "" {
		t.Fatalf("the refused Create left the branch: %q", got)
	}
}

// TestGitRefusesAfterBranchMade: when git fails after it made the branch,
// for example in a folder it cannot write to, the branch is removed.
func TestGitRefusesAfterBranchMade(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a folder that cannot be written to")
	}
	r := newRepo(t)
	dir := filepath.Join(r.Path, ".worktrees")
	if err := os.Mkdir(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if _, err := r.Create(context.Background(), "feat/ro"); err == nil {
		t.Fatal("Create in a read-only folder succeeded")
	}
	if got := runGit(t, r.Path, "branch", "--list", "feat/ro"); got != "" {
		t.Fatalf("the failed Create left the branch: %q", got)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Create(context.Background(), "feat/ro"); err != nil {
		t.Fatalf("the retry failed: %v", err)
	}
}

// TestEmptyFolderIsUsed: an empty folder where the worktree goes is fine,
// as git itself accepts it.
func TestEmptyFolderIsUsed(t *testing.T) {
	r := newRepo(t)
	if err := os.MkdirAll(filepath.Join(r.Path, ".worktrees", "feat-a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Create(context.Background(), "feat/a"); err != nil {
		t.Fatal(err)
	}
}

// TestPathsThroughASymlink: a caller that reaches the repository through a
// symlink still names its worktree. git lists the real paths, so Remove
// compares the real paths, not the strings.
func TestPathsThroughASymlink(t *testing.T) {
	r := newRepo(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(r.Path, link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	ctx := context.Background()
	via := Repo{Path: link}
	wt, err := via.Create(ctx, "feat/a")
	if err != nil {
		t.Fatal(err)
	}
	if err := via.Remove(ctx, filepath.Join(link, ".worktrees", "feat-a")); err != nil {
		t.Fatalf("Remove through the symlink: %v", err)
	}
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Fatalf("the worktree is still there: %v", err)
	}
}

// TestRemoveOnlySnowsFolder: a worktree the user made themselves, outside
// .worktrees, is never removed, even when it is clean and named by path.
func TestRemoveOnlySnowsFolder(t *testing.T) {
	r := newRepo(t)
	own := filepath.Join(t.TempDir(), "mine")
	runGit(t, r.Path, "worktree", "add", "-q", "-b", "mine", own)
	err := r.Remove(context.Background(), own)
	if !errors.Is(err, ErrNotWorktree) {
		t.Fatalf("error %v, want ErrNotWorktree", err)
	}
	if _, err := os.Stat(own); err != nil {
		t.Fatalf("the user's worktree is gone: %v", err)
	}
}
