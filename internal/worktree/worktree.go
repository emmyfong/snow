package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// dirName is the folder of the repository that holds the worktrees.
const dirName = ".worktrees"

// Create makes a worktree for branch, a new branch that starts at the
// current commit of the repository, in <repository>/.worktrees/<branch>
// with each character a Windows folder name cannot hold, such as a slash,
// as a dash. The main checkout, its index, and its branch stay as they are,
// and the repository config is not written. A branch that already exists, or
// a folder with files in it, is refused. If git fails after it made the
// branch, the branch is taken back, so a retry is not refused.
//
// The repository is the main checkout, also when r.Path is in a linked
// worktree or in a folder below the top. The returned path is the real path
// of the new worktree's top, as git lists it, so a caller that works in a
// subfolder joins it on.
func (r Repo) Create(ctx context.Context, branch string) (Worktree, error) {
	main, err := r.mainCheckout(ctx)
	if err != nil {
		return Worktree{}, err
	}
	if _, err := r.git(ctx, main.Path, "check-ref-format", "--branch", branch); err != nil {
		return Worktree{}, fmt.Errorf("%w: %q", ErrBadBranch, branch)
	}
	head, err := r.git(ctx, main.Path, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil {
		return Worktree{}, fmt.Errorf("%w: %w", ErrNoCommits, err)
	}
	if _, err := r.git(ctx, main.Path, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		return Worktree{}, fmt.Errorf("%w: %s", ErrBranchExists, branch)
	}
	dir := r.join(main.Path, dirName, slug(branch))
	if occupied(r.osPath(dir)) {
		return Worktree{}, fmt.Errorf("%w: %s", ErrPathExists, dir)
	}
	if err := r.ensureExcluded(ctx, main.Path); err != nil {
		return Worktree{}, err
	}
	if _, err := r.git(ctx, main.Path, "worktree", "add", "-b", branch, dir); err != nil {
		return Worktree{}, errors.Join(r.classify(err), r.undoBranch(ctx, main.Path, branch, strings.TrimSpace(head)))
	}
	return Worktree{Path: dir, Branch: branch, Head: strings.TrimSpace(head)}, nil
}

// Remove removes the worktree at path, and keeps its branch. It removes only
// a worktree in <repository>/.worktrees, never the main checkout and never
// one the user made elsewhere. It refuses a worktree with uncommitted
// changes or untracked files, and a detached one that holds commits no
// branch, remote branch, or tag reaches, so no work is lost. Files that git
// ignores, such as build output or a .env, are not work to git and go with
// the worktree. A path through a symlink is resolved before it is compared.
func (r Repo) Remove(ctx context.Context, path string) error {
	list, err := r.List(ctx)
	if err != nil {
		return err
	}
	var target *Worktree
	for i := range list {
		if r.same(list[i].Path, path) {
			target = &list[i]
		}
	}
	switch {
	case target == nil:
		return fmt.Errorf("%w: %s", ErrNotWorktree, path)
	case target.Main:
		return fmt.Errorf("%w: %s", ErrMainWorktree, path)
	case !r.same(r.dir(target.Path), r.join(list[0].Path, dirName)):
		// A worktree the user made is theirs to remove.
		return fmt.Errorf("%w: %s is not in %s", ErrNotWorktree, path, dirName)
	}
	status, err := r.git(ctx, target.Path, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return r.classify(err)
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("%w: %s", ErrDirty, target.Path)
	}
	if target.Detached || target.Branch == "" {
		// A branch keeps its commits when the worktree goes. A detached HEAD
		// does not: its new commits are reachable from nothing else.
		out, err := r.git(ctx, target.Path, "rev-list", "-n1", "HEAD", "--not", "--branches", "--remotes", "--tags")
		if err != nil {
			return r.classify(err)
		}
		if strings.TrimSpace(out) != "" {
			return fmt.Errorf("%w: %s", ErrUnreachableCommits, target.Path)
		}
	}
	// From the main checkout: the worktree's own folder is about to go.
	if _, err := r.git(ctx, list[0].Path, "worktree", "remove", target.Path); err != nil {
		return r.classify(err)
	}
	return nil
}

// undoBranch deletes the branch that a failed worktree add made, so the next
// try is not refused. It deletes only a branch that still points at head and
// that no worktree has checked out, so nothing of the user's goes.
func (r Repo) undoBranch(ctx context.Context, dir, branch, head string) error {
	ctx = context.WithoutCancel(ctx) // the cleanup runs after a cancel too
	tip, err := r.git(ctx, dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	if err != nil || strings.TrimSpace(tip) != head {
		return nil // the add made no branch, or someone moved it
	}
	list, err := r.List(ctx)
	if err != nil {
		return err
	}
	for _, w := range list {
		if w.Branch == branch {
			return nil
		}
	}
	if _, err := r.git(ctx, dir, "branch", "-d", branch); err != nil {
		return r.classify(err)
	}
	return nil
}

// occupied reports whether something is at p that git will not build a
// worktree in: a file, or a folder with files in it. An empty folder is fine.
func occupied(p string) bool {
	if entries, err := os.ReadDir(p); err == nil {
		return len(entries) > 0
	}
	_, err := os.Stat(p)
	return err == nil
}

// mainCheckout returns the repository's own checkout.
func (r Repo) mainCheckout(ctx context.Context) (Worktree, error) {
	list, err := r.List(ctx)
	if err != nil {
		return Worktree{}, err
	}
	if len(list) == 0 || list[0].Bare {
		return Worktree{}, fmt.Errorf("%w: %s has no checkout", ErrNotRepo, r.Path)
	}
	return list[0], nil
}

// slug turns a branch name into a folder name. Git allows names a Windows
// folder does not, such as a slash or a colon.
func slug(branch string) string {
	return strings.Map(func(c rune) rune {
		if strings.ContainsRune(`/\:*?"<>| `, c) {
			return '-'
		}
		return c
	}, branch)
}
